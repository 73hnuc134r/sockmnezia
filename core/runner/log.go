package runner

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	xlog "github.com/xtls/xray-core/common/log"
)

// Entry is one log line kept for the in-app log screen.
type Entry struct {
	Time    time.Time
	Level   string
	Message string
}

// Redactor masks secrets/hosts in log lines before they are stored (set by the app).
var Redactor = func(s string) string { return s }

const ringSize = 500

var (
	logMu      sync.Mutex
	ring       []Entry
	authFails  atomic.Int64
	minLevel   atomic.Int32 // xlog.Severity; lower is more severe
	logHandler = &handler{}
)

func init() { minLevel.Store(int32(xlog.Severity_Warning)) }

// SetLevel sets which Xray messages are kept ("error", "warning", "info", "debug").
func SetLevel(l string) {
	m := map[string]xlog.Severity{"error": xlog.Severity_Error, "warning": xlog.Severity_Warning,
		"info": xlog.Severity_Info, "debug": xlog.Severity_Debug}
	if s, ok := m[l]; ok {
		minLevel.Store(int32(s))
	}
}

// AuthFailures counts connections to the local proxy that failed authentication since process start.
func AuthFailures() int64 { return authFails.Load() }

// Logs returns the kept log lines, oldest first.
func Logs() []Entry {
	logMu.Lock()
	defer logMu.Unlock()
	return append([]Entry(nil), ring...)
}

// ClearLogs empties the ring.
func ClearLogs() {
	logMu.Lock()
	ring = nil
	logMu.Unlock()
}

// Record adds an app-side line to the same log.
func Record(level, msg string) {
	logMu.Lock()
	defer logMu.Unlock()
	ring = append(ring, Entry{Time: time.Now(), Level: level, Message: Redactor(msg)})
	if len(ring) > ringSize {
		ring = ring[len(ring)-ringSize:]
	}
}

type handler struct{}

// Severity returns Info so connection-level messages reach Handle (auth failures are logged at Info).
func (*handler) Severity() xlog.Severity { return xlog.Severity_Info }

func (*handler) Handle(msg xlog.Message) {
	s := msg.String()
	if strings.Contains(s, "invalid username or password") || strings.Contains(s, "no matching auth method") ||
		strings.Contains(s, "socks 4 is not allowed") {
		authFails.Add(1)
	}
	sev := xlog.Severity_Info
	level := "info"
	if gm, ok := msg.(*xlog.GeneralMessage); ok {
		sev = gm.Severity
		level = strings.ToLower(strings.TrimPrefix(gm.Severity.String(), "Severity_"))
	}
	if int32(sev) > minLevel.Load() {
		return
	}
	Record(level, s)
}

// attachLog installs our handler; Xray's app/log replaces the global handler on start, so this runs after.
func attachLog() { xlog.RegisterHandler(logHandler) }
