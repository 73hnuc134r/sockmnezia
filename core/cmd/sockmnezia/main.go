// Command sockmnezia is the desktop/Termux front end of the core: the same engine as the Android app.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nuc134r/sockmnezia/core/engine"
	"github.com/nuc134r/sockmnezia/core/routing"
	"github.com/nuc134r/sockmnezia/core/runner"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

const usage = `sockmnezia — local SOCKS5/HTTP proxy for Amnezia Premium (VLESS), optionally via your own server

Usage: sockmnezia [--dir DIR] <command> [args]

Amnezia:
  key [vpn://…]              set the subscription key (reads stdin when omitted)
  gateway-key FILE           find the gateway public key in an official APK / binary / PEM file
  fetch [--country CC]       get the Xray config from the gateway
  countries                  list locations from the last answer
Own server:
  own add LINK [NAME]        add vless/vmess/trojan/ss/socks/http link or Xray outbound JSON
  own list | own use ID | own rm ID
Settings:
  mode amnezia|amnezia_own|own
  set port N | user U | pass P | udp on|off | http PORT|off | listen IP | fetch-via-own on|off
  regen                      new random port, username and password
  routing show | routing preset ID [--append] | routing import FILE | routing test HOST [PORT] [tcp|udp]
  geo update | geo tags site|ip
Run:
  run                        start the proxy in the foreground (Ctrl+C stops)
  status | config | logs | forget
`

func defaultDir() string {
	if d := os.Getenv("SOCKMNEZIA_DIR"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "sockmnezia")
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "sockmnezia")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "state", "sockmnezia")
}

func die(err error) {
	var e *engine.Error
	if errors.As(err, &e) {
		fmt.Fprintf(os.Stderr, "error: %s\n", explain(e))
	} else {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(1)
}

func explain(e *engine.Error) string {
	hint := map[string]string{
		engine.CodeDeviceLimit:         "all device slots of the subscription are used; free one in the Amnezia Personal Dashboard",
		engine.CodeCaptchaRequired:     "the gateway wants a CAPTCHA; connect once in the official Amnezia app, then retry",
		engine.CodeSubscriptionExpired: "the subscription is not active or has expired",
		engine.CodeGatewayWrongKey:     "no gateway key worked (old APK?) or something on the network answered instead of the gateway",
		engine.CodeGatewayUnreachable:  "the gateway is unreachable; try 'set fetch-via-own on' with an own server",
		engine.CodeNoGatewayKey:        "run 'sockmnezia gateway-key <official APK or binary>' first",
		engine.CodeNoConfig:            "run 'sockmnezia fetch' first",
		engine.CodeGeoMissing:          "routing rules need geo files: run 'sockmnezia geo update'",
		engine.CodePortInUse:           "the port is in use; 'sockmnezia set port N' or 'sockmnezia regen'",
	}[e.Code]
	s := e.Code
	if e.Field != "" {
		s += " (" + e.Field + ")"
	}
	if hint != "" {
		s += ": " + hint
	}
	if e.Detail != "" {
		s += "\n  " + e.Detail
	}
	return s
}

func onOff(s string) bool {
	switch strings.ToLower(s) {
	case "on", "true", "yes", "1":
		return true
	case "off", "false", "no", "0":
		return false
	}
	die(fmt.Errorf("expected on or off, got %q", s))
	return false
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func main() {
	dir := flag.String("dir", defaultDir(), "state directory")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	e, err := engine.Open(*dir, engine.PlainBox{}, runtime.GOOS)
	if err != nil {
		die(err)
	}
	ctx := context.Background()
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		die(fmt.Errorf("missing argument for %s", args[0]))
		return ""
	}
	set := e.GetSettings()
	saveSet := func() {
		if err := e.SetSettings(set); err != nil {
			die(err)
		}
	}

	switch args[0] {
	case "key":
		k := ""
		if len(args) > 1 {
			k = args[1]
		} else {
			fmt.Fprint(os.Stderr, "Subscription key: ")
			k, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		}
		if err := e.SetKey(k); err != nil {
			die(err)
		}
		fmt.Println("key saved")
	case "gateway-key":
		path := arg(1)
		n, err := e.ImportGatewayKeysFromFile(path)
		if err != nil {
			die(err)
		}
		fmt.Printf("found %d key candidate(s); the working one is picked on the first fetch\n", n)
	case "fetch":
		fs := flag.NewFlagSet("fetch", flag.ExitOnError)
		country := fs.String("country", "", "location code")
		fs.Parse(args[1:])
		if err := e.Fetch(ctx, engine.FetchRequest{Country: *country}); err != nil {
			die(err)
		}
		s := e.Status()
		fmt.Printf("config for %s %s fetched; subscription until %s\n", s.Country, s.CountryName, s.SubscriptionTo)
	case "countries":
		for _, c := range e.Countries() {
			fmt.Printf("%s\t%s\n", c.Code, c.Name)
		}
	case "own":
		switch arg(1) {
		case "add":
			id, err := e.AddOwnServer(arg(2), strings.Join(args[min(3, len(args)):], " "))
			if err != nil {
				die(err)
			}
			fmt.Println("added", id)
		case "list":
			s := e.Snapshot()
			for _, o := range s.Own {
				mark := " "
				if o.ID == s.ActiveOwn {
					mark = "*"
				}
				fmt.Printf("%s %s  %s  %s %s:%d %s/%s\n", mark, o.ID, o.Name, o.Server.Protocol, o.Server.Address, o.Server.Port, o.Server.Network, o.Server.Security)
			}
		case "use":
			if err := e.SetActiveOwnServer(arg(2)); err != nil {
				die(err)
			}
		case "rm":
			if err := e.RemoveOwnServer(arg(2)); err != nil {
				die(err)
			}
		default:
			die(fmt.Errorf("own add|list|use|rm"))
		}
	case "mode":
		set.Mode = xrayconf.Mode(arg(1))
		saveSet()
	case "set":
		v := arg(2)
		switch arg(1) {
		case "port":
			set.Inbound.SocksPort, _ = strconv.Atoi(v)
		case "user":
			set.Inbound.User = v
		case "pass":
			set.Inbound.Pass = v
		case "udp":
			set.Inbound.UDP = onOff(v)
		case "http":
			if v == "off" {
				set.Inbound.HTTPPort = 0
			} else {
				set.Inbound.HTTPPort, _ = strconv.Atoi(v)
			}
		case "listen":
			set.Inbound.Listen = v
		case "fetch-via-own":
			set.Options.FetchViaOwn = onOff(v)
		default:
			die(fmt.Errorf("unknown setting %q", args[1]))
		}
		saveSet()
	case "regen":
		set.Inbound.SocksPort = engine.RandomPort(set.Inbound.Listen, set.Inbound.HTTPPort)
		set.Inbound.User, set.Inbound.Pass = engine.RandomUser(), engine.RandomPassword()
		saveSet()
		printProxy(e)
	case "routing":
		switch arg(1) {
		case "show":
			printJSON(set.Routing)
		case "preset":
			fs := flag.NewFlagSet("preset", flag.ExitOnError)
			app := fs.Bool("append", false, "append instead of replacing")
			fs.Parse(args[3:])
			r, ok := set.Routing.Apply(arg(2), !*app)
			if !ok {
				ids := []string{}
				for _, p := range routing.Presets {
					ids = append(ids, p.ID)
				}
				die(fmt.Errorf("presets: %s", strings.Join(ids, ", ")))
			}
			set.Routing = r
			saveSet()
		case "import":
			b, err := os.ReadFile(arg(2))
			if err != nil {
				die(err)
			}
			r, err := routing.ParseJSON(b)
			if err != nil {
				die(err)
			}
			set.Routing = r
			saveSet()
		case "test":
			port, net := 443, "tcp"
			if len(args) > 3 {
				port, _ = strconv.Atoi(args[3])
			}
			if len(args) > 4 {
				net = args[4]
			}
			v, err := e.TestRoute(arg(2), port, net)
			if err != nil {
				die(err)
			}
			printJSON(v)
		default:
			die(fmt.Errorf("routing show|preset|import|test"))
		}
	case "geo":
		switch arg(1) {
		case "update":
			if err := e.UpdateGeo(ctx); err != nil {
				die(err)
			}
			fmt.Println("geo files updated")
		case "tags":
			tags, err := e.GeoTags(arg(2))
			if err != nil {
				die(err)
			}
			fmt.Println(strings.Join(tags, "\n"))
		}
	case "run":
		if err := e.Start(ctx); err != nil {
			die(err)
		}
		printProxy(e)
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		tick := time.NewTicker(time.Minute)
		for {
			select {
			case <-sig:
				e.Stop()
				return
			case <-tick.C:
				s := e.Status()
				fmt.Fprintf(os.Stderr, "up %d KiB, down %d KiB, failed auth %d\n", s.Up/1024, s.Down/1024, s.AuthFailures)
			}
		}
	case "status":
		printJSON(e.Status())
	case "config":
		printProxy(e)
	case "logs":
		for _, l := range runner.Logs() {
			fmt.Printf("%s %s %s\n", l.Time.Format(time.TimeOnly), l.Level, l.Message)
		}
	case "forget":
		if err := e.Forget(); err != nil {
			die(err)
		}
		fmt.Println("all local data deleted (the device slot stays used until freed in the Personal Dashboard)")
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func printProxy(e *engine.Engine) {
	in := e.GetSettings().Inbound
	fmt.Printf("SOCKS5  %s:%d  user %s  password %s  (UDP %v)\n", in.Listen, in.SocksPort, in.User, in.Pass, in.UDP)
	fmt.Printf("        socks5://%s:%s@%s:%d\n", in.User, in.Pass, in.Listen, in.SocksPort)
	if in.HTTPPort != 0 {
		fmt.Printf("HTTP    %s:%d  (same credentials)\n", in.Listen, in.HTTPPort)
	}
}
