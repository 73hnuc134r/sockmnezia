package main

import (
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// Windows: the per-user WinINET settings (what Edge, Chrome and most apps use), then a notification so
// running apps reload them.
const inetKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type winProxy struct{}

func platformProxy() osProxy { return winProxy{} }

func (winProxy) Save() (map[string]string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, inetKey, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()
	m := map[string]string{}
	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		m["ProxyEnable"] = strconv.FormatUint(v, 10)
	}
	for _, n := range []string{"ProxyServer", "ProxyOverride", "AutoConfigURL"} {
		if v, _, err := k.GetStringValue(n); err == nil {
			m[n] = v
		}
	}
	return m, nil
}

func (winProxy) SetHTTP(hostport string, bypass []string) error {
	return setInet(map[string]string{"ProxyEnable": "1", "ProxyServer": hostport, "ProxyOverride": winBypass(bypass)})
}

func (winProxy) Restore(m map[string]string) error {
	if _, ok := m["ProxyEnable"]; !ok {
		m["ProxyEnable"] = "0"
	}
	return setInet(m)
}

// setInet writes the given values and deletes the other proxy values (absent = not set).
func setInet(m map[string]string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, inetKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	en, _ := strconv.ParseUint(m["ProxyEnable"], 10, 32)
	if err := k.SetDWordValue("ProxyEnable", uint32(en)); err != nil {
		return err
	}
	for _, n := range []string{"ProxyServer", "ProxyOverride", "AutoConfigURL"} {
		if v, ok := m[n]; ok {
			err = k.SetStringValue(n, v)
		} else {
			err = k.DeleteValue(n)
			if err == registry.ErrNotExist || err == syscall.ERROR_FILE_NOT_FOUND {
				err = nil
			}
		}
		if err != nil {
			return err
		}
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

var (
	wininet           = syscall.NewLazyDLL("wininet.dll")
	internetSetOption = wininet.NewProc("InternetSetOptionW")
)

// refreshInet tells WinINET users that the settings changed (INTERNET_OPTION_SETTINGS_CHANGED, _REFRESH).
func refreshInet() {
	internetSetOption.Call(0, 39, 0, 0)
	internetSetOption.Call(0, 37, 0, 0)
}
