package tun

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Linux: a systemd unit running as root, installed through pkexec (a polkit password prompt). The binary is
// copied to a root-only directory; the socket is in /run/sockmnezia, owned by the user who installed it.
// The nftables kill switch (table inet sockmnezia) drops everything that leaves outside the tun except
// Xray's own marked sockets and the local network, from the service's start until the uninstall: so a crash
// or a restart of the service never lets traffic around the tun.

const (
	unitPath   = "/etc/systemd/system/" + ServiceName + ".service"
	binDir     = "/usr/local/libexec/sockmnezia"
	socketPath = "/run/sockmnezia/tun.sock"
	stateDir   = "/var/lib/sockmnezia"
	nftTable   = "sockmnezia"
)

func dialService() (net.Conn, error) {
	c, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
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

// Installed reports whether the unit is installed.
func Installed() bool {
	_, err := os.Stat(unitPath)
	return err == nil
}

// runPrivileged runs this binary with flag as root through pkexec and waits for it.
func runPrivileged(flag string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("pkexec"); err != nil {
		return errors.New("pkexec (polkit) is needed to set up TUN mode")
	}
	out, err := exec.Command("pkexec", exe, flag).CombinedOutput()
	var ee *exec.ExitError
	if errors.As(err, &ee) && (ee.ExitCode() == 126 || ee.ExitCode() == 127) {
		return ErrDeclined
	}
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// callerUID is the user pkexec ran us for (or, with root running it directly, SOCKMNEZIA_TUN_UID).
func callerUID() (int, error) {
	if os.Geteuid() != 0 {
		return -1, errors.New("needs root (started through pkexec)")
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

// RunInstall is the root side of Install: copy the binary, write and (re)start the unit.
func RunInstall([]string) error {
	uid, err := callerUID()
	if err != nil {
		return err
	}
	if fi, err := os.Stat("/run/systemd/system"); err != nil || !fi.IsDir() {
		return errors.New("TUN mode needs systemd")
	}
	if _, err := exec.LookPath("nft"); err != nil {
		return errors.New("TUN mode needs nftables (the nft command)")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	bin := filepath.Join(binDir, "sockmnezia")
	exec.Command("systemctl", "stop", ServiceName).Run()
	if err := copyFile(exe, bin, 0o755); err != nil {
		return err
	}
	unit := `[Unit]
Description=sockmnezia TUN (installed by the sockmnezia app for TUN mode; removed when TUN mode is turned off)
Wants=network-pre.target
Before=network-pre.target

[Service]
ExecStart=` + bin + ` --tun-service --owner ` + strconv.Itoa(uid) + `
Restart=always
RestartSec=1
RuntimeDirectory=sockmnezia
StateDirectory=sockmnezia
StateDirectoryMode=0700

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", ServiceName}, {"restart", ServiceName}} {
		if out, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl %s: %v: %s", args[0], err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// RunUninstall is the root side of Uninstall: stop the unit (Xray closes, the routes go), remove it and the
// kill switch.
func RunUninstall([]string) error {
	if os.Geteuid() != 0 {
		return errors.New("needs root")
	}
	exec.Command("systemctl", "disable", "--now", ServiceName).Run()
	os.Remove(unitPath)
	exec.Command("systemctl", "daemon-reload").Run()
	killSwitchOff()
	os.RemoveAll(stateDir)
	os.RemoveAll(binDir)
	return nil
}

// RunService is the unit: kill switch, socket, the last state, then serve until SIGTERM.
func RunService(version string, args []string) error {
	if os.Geteuid() != 0 {
		return errors.New("the TUN service runs as root")
	}
	owner := -1
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--owner" {
			owner, _ = strconv.Atoi(args[i+1])
		}
	}
	if owner < 0 {
		return errors.New("--owner <uid> missing")
	}
	if err := killSwitchOn(); err != nil {
		return err
	}
	sdir := stateDir
	if d := os.Getenv("SOCKMNEZIA_TUN_STATE"); d != "" {
		sdir = d // tests
	}
	if err := os.MkdirAll(sdir, 0o700); err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(socketPath), 0o755)
	os.Remove(socketPath)
	old := syscall.Umask(0o177)
	l, err := net.Listen("unix", socketPath)
	syscall.Umask(old)
	if err != nil {
		return err
	}
	if err := os.Chown(socketPath, owner, -1); err != nil {
		l.Close()
		return err
	}
	s := &Service{Version: version, StateDir: sdir}
	if err := s.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "TUN service: last config:", err) // the kill switch still blocks
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		l.Close()
	}()
	s.Serve(&peerListener{Listener: l, uid: owner})
	s.Close()
	return nil
}

// peerListener accepts only the owner (and root).
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

// killSwitchOn installs the nftables rules (replacing earlier ones).
func killSwitchOn() error {
	rules := fmt.Sprintf(`table inet %[1]s
delete table inet %[1]s
table inet %[1]s {
	chain output {
		type filter hook output priority filter; policy accept;
		oifname "lo" accept
		oifname "sockmnezia" accept
		meta mark 0x%[2]x accept
		ip daddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, 224.0.0.0/4, 255.255.255.255 } accept
		ip6 daddr { fe80::/10, fc00::/7, ff00::/8 } accept
		counter drop
	}
}
`, nftTable, Mark)
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(rules)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func killSwitchOff() {
	exec.Command("nft", "delete", "table", "inet", nftTable).Run()
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
