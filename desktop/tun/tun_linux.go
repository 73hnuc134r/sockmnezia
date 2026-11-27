package tun

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// Linux: the helper runs as root through pkexec (a polkit password prompt). It computes its socket from the
// calling user (PKEXEC_UID), so its command line has no arguments to trust; both sides check the other's
// credentials on the socket. No-prompt mode installs a polkit rule that lets this user run exactly
// "<binary> --tun-helper" without the prompt.

func socketPath(uid int) string { return fmt.Sprintf("/run/user/%d/sockmnezia-tun.sock", uid) }

func rulePath(username string) string {
	return "/etc/polkit-1/rules.d/49-sockmnezia-" + username + ".rules"
}

func dialHelper() (net.Conn, error) {
	c, err := net.Dial("unix", socketPath(os.Getuid()))
	if err != nil {
		return nil, err
	}
	// Only a root helper may get the config: a socket left by another process of this user is refused.
	if uid, err := peerUID(c); err != nil || uid != 0 {
		c.Close()
		return nil, errors.New("the TUN socket is not served by root")
	}
	return c, nil
}

func peerUID(c net.Conn) (int, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return -1, errors.New("not a unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return -1, err
	}
	var cred *unix.Ucred
	var cerr error
	raw.Control(func(fd uintptr) { cred, cerr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED) })
	if cerr != nil {
		return -1, cerr
	}
	return int(cred.Uid), nil
}

// launchHelper starts "<binary> --tun-helper" through pkexec; done reports when pkexec exits.
func launchHelper() (<-chan error, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("pkexec"); err != nil {
		return nil, errors.New("pkexec (polkit) is needed for TUN mode")
	}
	cmd := exec.Command("pkexec", exe, "--tun-helper")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		var ee *exec.ExitError
		if errors.As(err, &ee) && (ee.ExitCode() == 126 || ee.ExitCode() == 127) {
			err = ErrDeclined // dismissed or not authorized
		}
		done <- err
	}()
	return done, nil
}

// helperUID is the user the helper serves: the one pkexec ran it for, or with root running it directly
// (tests), SOCKMNEZIA_TUN_UID.
func helperUID() (int, error) {
	if os.Geteuid() != 0 {
		return -1, errors.New("the TUN helper needs root (it is started through pkexec)")
	}
	s := os.Getenv("PKEXEC_UID")
	if s == "" {
		s = os.Getenv("SOCKMNEZIA_TUN_UID")
	}
	uid, err := strconv.Atoi(s)
	if err != nil || uid < 0 {
		return -1, errors.New("unknown calling user")
	}
	return uid, nil
}

func listenHelper(args []string) (net.Listener, bool, error) {
	uid, err := helperUID()
	if err != nil {
		return nil, false, err
	}
	path := socketPath(uid)
	if fi, err := os.Stat(filepath.Dir(path)); err != nil || !fi.IsDir() {
		return nil, false, fmt.Errorf("no runtime directory %s", filepath.Dir(path))
	}
	os.Remove(path)
	old := syscall.Umask(0o177)
	l, err := net.Listen("unix", path)
	syscall.Umask(old)
	if err != nil {
		return nil, false, err
	}
	if err := os.Chown(path, uid, -1); err != nil {
		l.Close()
		return nil, false, err
	}
	return &peerListener{Listener: l, uid: uid}, false, nil
}

// peerListener accepts only the user it serves (and root).
type peerListener struct {
	net.Listener
	uid int
}

func (p *peerListener) Accept() (net.Conn, error) {
	for {
		c, err := p.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if uid, err := peerUID(c); err == nil && (uid == p.uid || uid == 0) {
			return c, nil
		}
		c.Close()
	}
}

// NoPromptInstalled reports whether no-prompt mode is set up for this user.
func NoPromptInstalled() bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	_, err = os.Stat(rulePath(u.Username))
	return err == nil
}

// SetNoPrompt installs or removes the polkit rule (one password prompt).
func SetNoPrompt(on bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	flag := "--tun-noprompt-off"
	if on {
		flag = "--tun-noprompt-on"
	}
	err = exec.Command("pkexec", exe, flag).Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && (ee.ExitCode() == 126 || ee.ExitCode() == 127) {
		return ErrDeclined
	}
	return err
}

// RunNoPrompt is the privileged side of SetNoPrompt, for the user pkexec ran it for.
func RunNoPrompt(on bool, _ []string) error {
	uid, err := helperUID()
	if err != nil {
		return err
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return err
	}
	path := rulePath(u.Username)
	if !on {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(polkitRule(exe, u.Username)), 0o644)
}

func polkitRule(exe, username string) string {
	q := func(s string) string { return strconv.Quote(s) }
	return `// sockmnezia: lets ` + username + ` start the TUN helper without a password (turn off in the app).
polkit.addRule(function(action, subject) {
    if (action.id == "org.freedesktop.policykit.exec" &&
        action.lookup("program") == ` + q(exe) + ` &&
        action.lookup("command_line") == ` + q(exe+" --tun-helper") + ` &&
        subject.user == ` + q(username) + ` && subject.local && subject.active) {
        return polkit.Result.YES;
    }
});
`
}
