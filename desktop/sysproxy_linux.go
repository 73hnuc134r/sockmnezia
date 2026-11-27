package main

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"strings"
)

// Linux: GNOME-style GSettings (GNOME, Cinnamon, Budgie, and what GTK/Chromium read on many desktops) and
// KDE's kioslaverc, whichever exist. Values are saved and restored in their own text form.

type linuxProxy struct {
	gnome bool
	kde   string // kwriteconfig6 / kwriteconfig5, "" = not KDE
}

func platformProxy() osProxy {
	p := linuxProxy{}
	if exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode").Run() == nil {
		p.gnome = true
	}
	if strings.Contains(strings.ToUpper(os.Getenv("XDG_CURRENT_DESKTOP")), "KDE") {
		for _, k := range []string{"kwriteconfig6", "kwriteconfig5"} {
			if _, err := exec.LookPath(k); err == nil {
				p.kde = k
				break
			}
		}
	}
	if !p.gnome && p.kde == "" {
		return nil
	}
	return p
}

var gnomeKeys = [][2]string{
	{"org.gnome.system.proxy", "mode"}, {"org.gnome.system.proxy", "autoconfig-url"}, {"org.gnome.system.proxy", "ignore-hosts"},
	{"org.gnome.system.proxy.http", "host"}, {"org.gnome.system.proxy.http", "port"},
	{"org.gnome.system.proxy.https", "host"}, {"org.gnome.system.proxy.https", "port"},
}

var kdeKeys = []string{"ProxyType", "httpProxy", "httpsProxy", "Proxy Config Script", "NoProxyFor"}

func (p linuxProxy) Save() (map[string]string, error) {
	m := map[string]string{}
	if p.gnome {
		for _, k := range gnomeKeys {
			out, err := exec.Command("gsettings", "get", k[0], k[1]).Output()
			if err != nil {
				return nil, err
			}
			m["gnome:"+k[0]+" "+k[1]] = strings.TrimSpace(string(out))
		}
	}
	if p.kde != "" {
		read := strings.Replace(p.kde, "kwriteconfig", "kreadconfig", 1)
		for _, k := range kdeKeys {
			out, _ := exec.Command(read, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", k).Output()
			m["kde:"+k] = strings.TrimSpace(string(out))
		}
	}
	return m, nil
}

func (p linuxProxy) SetHTTP(hostport string, bypass []string) error {
	host, port, _ := net.SplitHostPort(hostport)
	var errs []error
	if p.gnome {
		ign := make([]string, len(bypass))
		for i, b := range bypass {
			ign[i] = "'" + b + "'"
		}
		errs = append(errs, gset("org.gnome.system.proxy", "ignore-hosts", "["+strings.Join(ign, ", ")+"]"),
			gset("org.gnome.system.proxy.http", "host", host), gset("org.gnome.system.proxy.http", "port", port),
			gset("org.gnome.system.proxy.https", "host", host), gset("org.gnome.system.proxy.https", "port", port),
			gset("org.gnome.system.proxy", "mode", "manual"))
	}
	if p.kde != "" {
		url := "http://" + host + " " + port
		errs = append(errs, p.kset("httpProxy", url), p.kset("httpsProxy", url), p.kset("NoProxyFor", strings.Join(bypass, ",")), p.kset("ProxyType", "1"))
		kdeReload()
	}
	return errors.Join(errs...)
}

func (p linuxProxy) Restore(m map[string]string) error {
	var errs []error
	for k, v := range m {
		if g, ok := strings.CutPrefix(k, "gnome:"); ok && p.gnome {
			schema, key, _ := strings.Cut(g, " ")
			errs = append(errs, gsetRaw(schema, key, v))
		}
		if key, ok := strings.CutPrefix(k, "kde:"); ok && p.kde != "" {
			if v == "" {
				exec.Command(p.kde, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", key, "--delete").Run()
			} else {
				errs = append(errs, p.kset(key, v))
			}
		}
	}
	if p.kde != "" {
		kdeReload()
	}
	return errors.Join(errs...)
}

// gset sets a string or number key (quoted as a GVariant string when it is not a number).
func gset(schema, key, v string) error {
	if strings.Trim(v, "0123456789") != "" && !strings.HasPrefix(v, "[") {
		v = "'" + strings.ReplaceAll(v, "'", "") + "'"
	}
	return gsetRaw(schema, key, v)
}

// gsetRaw sets a key from its GVariant text (as gsettings get prints it).
func gsetRaw(schema, key, v string) error {
	return exec.Command("gsettings", "set", schema, key, v).Run()
}

func (p linuxProxy) kset(key, v string) error {
	return exec.Command(p.kde, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", key, v).Run()
}

func kdeReload() {
	exec.Command("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:").Run()
}
