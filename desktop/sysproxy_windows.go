package main

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows: the per-connection WinINET proxy settings of the LAN connection, set through
// InternetSetOption(INTERNET_OPTION_PER_CONNECTION_OPTION), as v2rayN does. That is what the Settings app shows
// and what Edge, Chrome and WinHTTP read (Connections\DefaultConnectionSettings); the older ProxyEnable /
// ProxyServer registry values alone are not (D42). Running apps are told to reload them.

const (
	optRefresh         = 37 // INTERNET_OPTION_REFRESH
	optSettingsChanged = 39 // INTERNET_OPTION_SETTINGS_CHANGED
	optPerConnection   = 75 // INTERNET_OPTION_PER_CONNECTION_OPTION

	perConnFlags     = 1 // INTERNET_PER_CONN_FLAGS
	perConnServer    = 2 // INTERNET_PER_CONN_PROXY_SERVER
	perConnBypass    = 3 // INTERNET_PER_CONN_PROXY_BYPASS
	perConnAutoURL   = 4 // INTERNET_PER_CONN_AUTOCONFIG_URL
	proxyTypeDirect  = 1 // PROXY_TYPE_DIRECT
	proxyTypeProxy   = 2 // PROXY_TYPE_PROXY
	proxyTypeAutoURL = 4 // PROXY_TYPE_AUTO_PROXY_URL
)

// perConnOption is INTERNET_PER_CONN_OPTIONW: a DWORD and a union of DWORD / LPWSTR / FILETIME (8 bytes).
type perConnOption struct {
	option uint32
	value  uintptr
}

// perConnList is INTERNET_PER_CONN_OPTION_LISTW; a nil connection is the LAN connection.
type perConnList struct {
	size        uint32
	connection  *uint16
	optionCount uint32
	optionError uint32
	options     *perConnOption
}

var (
	wininet             = syscall.NewLazyDLL("wininet.dll")
	internetSetOption   = wininet.NewProc("InternetSetOptionW")
	internetQueryOption = wininet.NewProc("InternetQueryOptionW")
	globalFree          = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalFree")
)

type winProxy struct{}

func platformProxy() osProxy { return winProxy{} }

// Save reads the current settings: flags, server, bypass list and auto-config URL.
func (winProxy) Save() (map[string]string, error) {
	opts := []perConnOption{{option: perConnFlags}, {option: perConnServer}, {option: perConnBypass}, {option: perConnAutoURL}}
	list := perConnList{optionCount: uint32(len(opts)), options: &opts[0]}
	list.size = uint32(unsafe.Sizeof(list))
	size := list.size
	r, _, err := internetQueryOption.Call(0, optPerConnection, uintptr(unsafe.Pointer(&list)), uintptr(unsafe.Pointer(&size)))
	runtime.KeepAlive(opts)
	if r == 0 {
		return nil, err
	}
	str := func(o *perConnOption) string {
		p := *(**uint16)(unsafe.Pointer(&o.value)) // LPWSTR from GlobalAlloc
		if p == nil {
			return ""
		}
		s := windows.UTF16PtrToString(p)
		globalFree.Call(o.value)
		return s
	}
	return map[string]string{
		"flags":  strconv.FormatUint(uint64(opts[0].value), 10),
		"server": str(&opts[1]),
		"bypass": str(&opts[2]),
		"pac":    str(&opts[3]),
	}, nil
}

func (winProxy) SetHTTP(hostport string, bypass []string) error {
	return setPerConn(proxyTypeDirect|proxyTypeProxy, hostport, winBypass(bypass), "")
}

// Restore puts back what Save returned (or, from versions before D42, the registry values they saved).
func (winProxy) Restore(m map[string]string) error {
	if _, old := m["ProxyEnable"]; old {
		flags := uint32(proxyTypeDirect)
		if m["ProxyEnable"] == "1" {
			flags |= proxyTypeProxy
		}
		if m["AutoConfigURL"] != "" {
			flags |= proxyTypeAutoURL
		}
		return setPerConn(flags, m["ProxyServer"], m["ProxyOverride"], m["AutoConfigURL"])
	}
	flags, err := strconv.ParseUint(m["flags"], 10, 32)
	if err != nil || flags == 0 {
		flags = proxyTypeDirect
	}
	return setPerConn(uint32(flags), m["server"], m["bypass"], m["pac"])
}

// setPerConn sets the LAN connection's proxy settings and tells running apps.
func setPerConn(flags uint32, server, bypass, pac string) error {
	var keep []*uint16
	ptr := func(s string) uintptr {
		p, err := windows.UTF16PtrFromString(s)
		if err != nil {
			p, _ = windows.UTF16PtrFromString("")
		}
		keep = append(keep, p)
		return uintptr(unsafe.Pointer(p))
	}
	opts := []perConnOption{
		{option: perConnFlags, value: uintptr(flags)},
		{option: perConnServer, value: ptr(server)},
		{option: perConnBypass, value: ptr(bypass)},
		{option: perConnAutoURL, value: ptr(pac)},
	}
	list := perConnList{optionCount: uint32(len(opts)), options: &opts[0]}
	list.size = uint32(unsafe.Sizeof(list))
	r, _, err := internetSetOption.Call(0, optPerConnection, uintptr(unsafe.Pointer(&list)), uintptr(list.size))
	runtime.KeepAlive(opts)
	runtime.KeepAlive(keep)
	if r == 0 {
		if err == nil || errors.Is(err, windows.ERROR_SUCCESS) {
			err = errors.New("InternetSetOption failed")
		}
		return err
	}
	refreshInet()
	return nil
}

// winBypass turns CIDRs into WinINET's wildcard form; "<local>" covers names without dots.
func winBypass(l []string) string {
	out := []string{"<local>"}
	for _, b := range l {
		switch b {
		case "127.0.0.0/8":
			out = append(out, "127.*")
		case "10.0.0.0/8":
			out = append(out, "10.*")
		case "192.168.0.0/16":
			out = append(out, "192.168.*")
		case "169.254.0.0/16":
			out = append(out, "169.254.*")
		case "172.16.0.0/12":
			for i := 16; i <= 31; i++ {
				out = append(out, "172."+strconv.Itoa(i)+".*")
			}
		default:
			if !strings.Contains(b, "/") {
				out = append(out, b)
			}
		}
	}
	return strings.Join(out, ";")
}

// refreshInet tells WinINET users that the settings changed.
func refreshInet() {
	internetSetOption.Call(0, optSettingsChanged, 0, 0)
	internetSetOption.Call(0, optRefresh, 0, 0)
}
