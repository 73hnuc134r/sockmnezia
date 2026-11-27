package tun

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// Windows: the helper runs elevated, started through a UAC prompt (ShellExecute "runas"). It serves a named
// pipe that only the calling user's account (and SYSTEM) may open; the app checks the pipe's server is an
// elevated process before sending anything. No-prompt mode registers a Task Scheduler task that starts the
// helper with the highest privileges at logon (as v2rayN does for the whole app), which the app can also
// start on demand.

const taskName = `sockmnezia TUN`

func userSID() (string, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

func pipeName(sid string) string { return `\\.\pipe\sockmnezia-tun-` + sid }

func dialHelper() (net.Conn, error) {
	sid, err := userSID()
	if err != nil {
		return nil, err
	}
	timeout := 2 * time.Second
	c, err := winio.DialPipe(pipeName(sid), &timeout)
	if err != nil {
		return nil, err
	}
	if !serverElevated(c) {
		c.Close()
		return nil, errors.New("the TUN pipe is not served by an elevated process")
	}
	return c, nil
}

// serverElevated checks the process at the other end of the pipe runs with administrator rights, so a
// process of this user that took the pipe name first never gets the config.
func serverElevated(c net.Conn) bool {
	f, ok := c.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	var pid uint32
	if windows.GetNamedPipeServerProcessId(windows.Handle(f.Fd()), &pid) != nil {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var tok windows.Token
	if windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok) != nil {
		return false
	}
	defer tok.Close()
	return tok.IsElevated()
}

func launchHelper() (<-chan error, error) {
	sid, err := userSID()
	if err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	// No-prompt mode: run the logon task now (allowed for the user it runs as).
	if NoPromptInstalled() {
		cmd := exec.Command("schtasks", "/Run", "/TN", taskName)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if cmd.Run() == nil {
			go func() { time.Sleep(20 * time.Second); done <- nil }()
			return done, nil
		}
	}
	if err := runElevated("--tun-helper --sid " + sid); err != nil {
		return nil, err
	}
	go func() { time.Sleep(110 * time.Second); done <- nil }()
	return done, nil
}

// runElevated starts this binary with args through a UAC prompt.
func runElevated(args string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	a, _ := windows.UTF16PtrFromString(args)
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(exe))
	err = windows.ShellExecute(0, verb, file, a, dir, windows.SW_HIDE)
	if errors.Is(err, windows.ERROR_CANCELLED) {
		return ErrDeclined
	}
	return err
}

func listenHelper(args []string) (net.Listener, bool, error) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return nil, false, errors.New("the TUN helper needs administrator rights")
	}
	sid, persistent := "", false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--sid":
			if i+1 < len(args) {
				sid = args[i+1]
				i++
			}
		case "--persistent":
			persistent = true
		}
	}
	if _, err := windows.StringToSid(sid); err != nil || !strings.HasPrefix(sid, "S-1-") {
		return nil, false, fmt.Errorf("invalid user SID %q", sid)
	}
	l, err := winio.ListenPipe(pipeName(sid), &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + sid + ")(A;;GA;;;SY)",
		InputBufferSize:    64 << 10, OutputBufferSize: 64 << 10,
	})
	return l, persistent, err
}

// NoPromptInstalled reports whether the logon task exists.
func NoPromptInstalled() bool {
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run() == nil
}

// SetNoPrompt registers or deletes the logon task (one UAC prompt).
func SetNoPrompt(on bool) error {
	sid, err := userSID()
	if err != nil {
		return err
	}
	if on {
		return runElevated("--tun-noprompt-on --sid " + sid)
	}
	return runElevated("--tun-noprompt-off --sid " + sid)
}

// RunNoPrompt is the elevated side of SetNoPrompt.
func RunNoPrompt(on bool, args []string) error {
	sid := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--sid" {
			sid = args[i+1]
		}
	}
	if _, err := windows.StringToSid(sid); err != nil {
		return fmt.Errorf("invalid user SID %q", sid)
	}
	if !on {
		return schtasks("/Delete", "/TN", taskName, "/F")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "sockmnezia-task-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	// schtasks reads UTF-16 task XML.
	u16, _ := windows.UTF16FromString(taskXML(exe, sid))
	b := []byte{0xff, 0xfe}
	for _, r := range u16[:len(u16)-1] {
		b = append(b, byte(r), byte(r>>8))
	}
	f.Write(b)
	f.Close()
	return schtasks("/Create", "/TN", taskName, "/XML", f.Name(), "/F")
}

func schtasks(args ...string) error {
	cmd := exec.Command("schtasks", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

func taskXML(exe, sid string) string {
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Description>Starts the sockmnezia TUN helper with administrator rights at logon, so TUN mode needs no prompt.</Description></RegistrationInfo>
  <Triggers><LogonTrigger><Enabled>true</Enabled><UserId>` + sid + `</UserId></LogonTrigger></Triggers>
  <Principals><Principal id="Author"><UserId>` + sid + `</UserId><LogonType>InteractiveToken</LogonType><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Priority>5</Priority>
  </Settings>
  <Actions Context="Author"><Exec><Command>"` + xmlEscape(exe) + `"</Command><Arguments>--tun-helper --sid ` + sid + ` --persistent</Arguments></Exec></Actions>
</Task>
`
}
