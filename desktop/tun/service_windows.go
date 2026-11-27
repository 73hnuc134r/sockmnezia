package tun

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Windows: a LocalSystem service, installed through a UAC prompt (ShellExecute "runas"). Its binary (with
// wintun.dll) is a copy in %ProgramData%\sockmnezia\bin, writable only by administrators; its state is in
// %ProgramData%\sockmnezia\state, readable only by them. It serves a named pipe that only the installing
// user (and SYSTEM) may open; the app checks the pipe is served by SYSTEM before sending anything. Windows
// restarts it after a crash; Xray's WFP filters (allexceptlan) keep traffic from leaving outside the tun
// while it runs, also while it blocks everything (the proxy stopped in TUN mode).

const pipeName = `\\.\pipe\sockmnezia-tun`

// adminOnly: full access for SYSTEM and Administrators, inherited, nothing else.
const adminOnly = "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"

func programData() string {
	if d := os.Getenv("ProgramData"); d != "" {
		return filepath.Join(d, "sockmnezia")
	}
	return `C:\ProgramData\sockmnezia`
}

func dialService() (net.Conn, error) {
	timeout := 2 * time.Second
	c, err := winio.DialPipe(pipeName, &timeout)
	if err != nil {
		return nil, err
	}
	if !servedBySystem(c) {
		c.Close()
		return nil, errors.New("the TUN pipe is not served by the system service")
	}
	return c, nil
}

// servedBySystem checks the process at the other end of the pipe runs as LocalSystem, so a program of this
// user that took the pipe name first never gets the config.
func servedBySystem(c net.Conn) bool {
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
	u, err := tok.GetTokenUser()
	return err == nil && u.User.Sid.IsWellKnown(windows.WinLocalSystemSid)
}

// Installed reports whether the service exists (no administrator rights needed to ask).
func Installed() bool {
	m, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return false
	}
	defer windows.CloseServiceHandle(m)
	name, _ := windows.UTF16PtrFromString(ServiceName)
	s, err := windows.OpenService(m, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return false
	}
	windows.CloseServiceHandle(s)
	return true
}

func userSID() (string, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

// runPrivileged starts this binary with flag through a UAC prompt (it then runs on its own).
func runPrivileged(flag string) error {
	sid, err := userSID()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	args, _ := windows.UTF16PtrFromString(flag + " --owner " + sid)
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(exe))
	err = windows.ShellExecute(0, verb, file, args, dir, windows.SW_HIDE)
	if errors.Is(err, windows.ERROR_CANCELLED) {
		return ErrDeclined
	}
	return err
}

func ownerArg(args []string) (string, error) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--owner" {
			sid := args[i+1]
			if _, err := windows.StringToSid(sid); err != nil || !strings.HasPrefix(sid, "S-1-") {
				return "", fmt.Errorf("invalid user SID %q", sid)
			}
			return sid, nil
		}
	}
	return "", errors.New("--owner <SID> missing")
}

// secureDir creates dir readable and writable only by SYSTEM and Administrators.
func secureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString(adminOnly)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

// RunInstall is the elevated side of Install: copy the binary and wintun.dll, create or update the service,
// start it.
func RunInstall(args []string) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("needs administrator rights")
	}
	sid, err := ownerArg(args)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	root := programData()
	if err := secureDir(root); err != nil {
		return err
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(ServiceName)
	if err == nil { // update: stop it so its binary can be replaced
		stopAndWait(s)
	}
	target := filepath.Join(bin, "sockmnezia.exe")
	if err := copyFile(exe, target); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(filepath.Dir(exe), "wintun.dll"), filepath.Join(bin, "wintun.dll")); err != nil {
		return fmt.Errorf("wintun.dll next to sockmnezia.exe: %w", err)
	}
	cfg := mgr.Config{
		DisplayName:  "sockmnezia TUN",
		Description:  "Runs sockmnezia's TUN mode. Installed by the sockmnezia app when TUN mode is chosen, removed when another mode is chosen.",
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	}
	if s == nil {
		s, err = m.CreateService(ServiceName, target, cfg, "--tun-service", "--owner", sid)
		if err != nil {
			return err
		}
	} else {
		cfg.BinaryPathName = fmt.Sprintf(`"%s" --tun-service --owner %s`, target, sid)
		if err := s.UpdateConfig(cfg); err != nil {
			s.Close()
			return err
		}
	}
	defer s.Close()
	restart := mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: time.Second}
	s.SetRecoveryActions([]mgr.RecoveryAction{restart, restart, restart}, 24*60*60)
	return s.Start()
}

func stopAndWait(s *mgr.Service) {
	s.Control(svc.Stop)
	for i := 0; i < 100; i++ {
		st, err := s.Query()
		if err != nil || st.State == svc.Stopped {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// RunUninstall is the elevated side of Uninstall: stop the service (Xray closes, its routes and filters go)
// and remove it with its files.
func RunUninstall([]string) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("needs administrator rights")
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s, err := m.OpenService(ServiceName); err == nil {
		stopAndWait(s)
		err = s.Delete()
		s.Close()
		if err != nil {
			return err
		}
	}
	time.Sleep(500 * time.Millisecond) // the binary is released after the process exits
	return os.RemoveAll(programData())
}

// RunService is the service process.
func RunService(version string, args []string) error {
	sid, err := ownerArg(args)
	if err != nil {
		return err
	}
	if is, _ := svc.IsWindowsService(); !is {
		return errors.New("runs as a Windows service only")
	}
	return svc.Run(ServiceName, &handler{version: version, owner: sid})
}

type handler struct{ version, owner string }

func (h *handler) Execute(_ []string, req <-chan svc.ChangeRequest, st chan<- svc.Status) (bool, uint32) {
	st <- svc.Status{State: svc.StartPending}
	state := filepath.Join(programData(), "state")
	secureDir(programData())
	os.MkdirAll(state, 0o700)
	l, err := winio.ListenPipe(pipeName, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + h.owner + ")(A;;GA;;;SY)",
		InputBufferSize:    64 << 10, OutputBufferSize: 64 << 10,
	})
	if err != nil {
		return false, 1
	}
	s := &Service{Version: h.version, StateDir: state}
	s.Load() // the last config, or it blocking everything; a broken one leaves nothing running
	go s.Serve(l)
	st <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range req {
		switch c.Cmd {
		case svc.Interrogate:
			st <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			st <- svc.Status{State: svc.StopPending}
			l.Close()
			s.Close()
			return false, 0
		}
	}
	return false, 0
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
