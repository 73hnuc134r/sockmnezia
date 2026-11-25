package engine

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nuc134r/sockmnezia/core/account"
	"github.com/nuc134r/sockmnezia/core/gateway"
	"github.com/nuc134r/sockmnezia/core/links"
	"github.com/nuc134r/sockmnezia/core/routing"
	"github.com/nuc134r/sockmnezia/core/xrayconf"
)

// SecretBox encrypts the state file at rest. On Android it is backed by a Keystore key; the CLI uses
// PlainBox (file mode 0600).
type SecretBox interface {
	Seal(plain []byte) ([]byte, error)
	Open(sealed []byte) ([]byte, error)
}

// PlainBox stores the state unencrypted (rely on file permissions).
type PlainBox struct{}

func (PlainBox) Seal(p []byte) ([]byte, error) { return p, nil }
func (PlainBox) Open(s []byte) ([]byte, error) { return s, nil }

// OwnServer is a saved own server.
type OwnServer struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Link   string        `json:"link"` // as imported (secret: contains credentials)
	Server *links.Server `json:"server"`
}

// Hop kinds in a saved chain.
const (
	HopAmnezia = "amnezia"
	HopServer  = "server"
)

// ChainHop is one hop of a saved chain: Amnezia, or an own server by ID.
type ChainHop struct {
	Kind   string `json:"kind"`
	Server string `json:"server,omitempty"` // OwnServer.ID when Kind is HopServer
}

// Chain is a saved, named chain of hops (phone first, exit last).
type Chain struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	Hops []ChainHop `json:"hops"`
}

// Options are the remaining settings.
type Options struct {
	// FetchVia is an own server ID the gateway request goes through when the active chain has no hops
	// before Amnezia ("" = direct). Hops before Amnezia are always used when there are any.
	FetchVia          string `json:"fetch_via,omitempty"`
	LegacyFetchViaOwn bool   `json:"fetch_via_own,omitempty"` // state v1; migrated to FetchVia
	GeoSource         string `json:"geo_source"`              // geo.Sources ID or "custom"
	GeoCustomURL      string `json:"geo_custom_url"`          // https base URL when GeoSource is "custom"
	GeoAutoUpdate     bool   `json:"geo_auto_update"`         // weekly, scheduled by the app
	LogLevel          string `json:"log_level"`               // warning by default
	ExitIPURL         string `json:"exit_ip_url"`             // plain-text "what is my IP" endpoint
	RefreshAfterHrs   int    `json:"refresh_after_hrs"`       // re-fetch the Amnezia config on start when older (0 = never)
}

// Amnezia is the subscription side of the state.
type Amnezia struct {
	Key         string           `json:"key,omitempty"` // the vpn:// subscription key (secret)
	Identity    account.Identity `json:"identity"`
	GatewayPEMs []string         `json:"gateway_pems,omitempty"` // candidates; the working one first
	GatewayURL  string           `json:"gateway_url,omitempty"`  // empty = production
	Country     string           `json:"country,omitempty"`      // requested location
	Last        *account.Result  `json:"last,omitempty"`
	FetchedAt   time.Time        `json:"fetched_at,omitempty"`
}

// State is everything persisted.
type State struct {
	Version     int              `json:"version"`
	Amnezia     Amnezia          `json:"amnezia"`
	Own         []OwnServer      `json:"own"`
	Chains      []Chain          `json:"chains"`
	ActiveChain string           `json:"active_chain,omitempty"`
	Inbound     xrayconf.Inbound `json:"inbound"`
	Routing     routing.Settings `json:"routing"`
	Options     Options          `json:"options"`

	// State v1 (three fixed modes); migrated to Chains on load.
	LegacyMode      xrayconf.Mode `json:"mode,omitempty"`
	LegacyActiveOwn string        `json:"active_own,omitempty"`
}

const stateVersion = 2

const stateFile = "state.bin"

// Ports that proxy clients use by default; a random port never lands on them.
var wellKnownPorts = map[int]bool{1080: true, 1081: true, 2080: true, 2081: true, 3128: true, 7890: true, 7891: true,
	8080: true, 8118: true, 8888: true, 9050: true, 9150: true, 10808: true, 10809: true, 10810: true, 10853: true, 20170: true, 20171: true}

const (
	portMin = 20000
	portMax = 60999
)

func randInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}

// RandomPort picks a free TCP port on host from 20000-60999, avoiding well-known proxy ports and the
// ports in avoid.
func RandomPort(host string, avoid ...int) int {
	skip := map[int]bool{}
	for _, a := range avoid {
		skip[a] = true
	}
	for i := 0; i < 200; i++ {
		p := portMin + randInt(portMax-portMin+1)
		if wellKnownPorts[p] || skip[p] {
			continue
		}
		if l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(p))); err == nil {
			l.Close()
			return p
		}
	}
	return portMin + randInt(portMax-portMin+1)
}

const (
	userAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	passAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
)

func randString(alphabet string, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[randInt(len(alphabet))]
	}
	return string(b)
}

// RandomUser returns a 16-character username.
func RandomUser() string { return randString(userAlphabet, 16) }

// RandomPassword returns a 32-character password (~187 bits).
func RandomPassword() string { return randString(passAlphabet, 32) }

func newID() string { return gateway.NewUUID() }

func amneziaChain() Chain {
	return Chain{ID: newID(), Name: "Amnezia", Hops: []ChainHop{{Kind: HopAmnezia}}}
}

func defaultState() *State {
	amz := amneziaChain()
	return &State{
		Version:     stateVersion,
		Chains:      []Chain{amz},
		ActiveChain: amz.ID,
		Amnezia:     Amnezia{Identity: account.NewIdentity()},
		Inbound: xrayconf.Inbound{
			Listen:    "127.0.0.1",
			SocksPort: RandomPort("127.0.0.1"),
			User:      RandomUser(),
			Pass:      RandomPassword(),
		},
		Routing: routing.Default(),
		Options: Options{
			GeoSource:       "runetfreedom",
			LogLevel:        "warning",
			ExitIPURL:       "https://api.ipify.org",
			RefreshAfterHrs: 24,
		},
	}
}

func load(dir string, box SecretBox) (*State, error) {
	b, err := os.ReadFile(filepath.Join(dir, stateFile))
	if errors.Is(err, os.ErrNotExist) {
		return defaultState(), nil
	}
	if err != nil {
		return nil, err
	}
	plain, err := box.Open(b)
	if err != nil {
		return nil, fmt.Errorf("state cannot be decrypted: %w", err)
	}
	st := defaultState()
	st.Version, st.Chains, st.ActiveChain = 0, nil, "" // the file says what it has; a v1 file has no chains
	if err := json.Unmarshal(plain, st); err != nil {
		return nil, fmt.Errorf("state is corrupt: %w", err)
	}
	migrate(st)
	if !st.Amnezia.Identity.Valid() {
		st.Amnezia.Identity = account.NewIdentity()
	}
	return st, nil
}

// migrate upgrades an older state in place. v1 -> v2: the three modes become saved chains built from the
// server that was active ("Amnezia", "Amnezia → <server>", "<server>"), and the chain matching the old mode
// becomes active. "Fetch through my own server" becomes "fetch through <that server>".
func migrate(st *State) {
	if st.Version < 2 {
		amz := amneziaChain()
		st.Chains, st.ActiveChain = []Chain{amz}, amz.ID
		var own *OwnServer
		for i := range st.Own {
			if st.Own[i].ID == st.LegacyActiveOwn {
				own = &st.Own[i]
			}
		}
		if own == nil && len(st.Own) > 0 {
			own = &st.Own[0]
		}
		if own != nil {
			srv := ChainHop{Kind: HopServer, Server: own.ID}
			via := Chain{ID: newID(), Name: "Amnezia → " + own.Name, Hops: []ChainHop{{Kind: HopAmnezia}, srv}}
			only := Chain{ID: newID(), Name: own.Name, Hops: []ChainHop{srv}}
			st.Chains = append(st.Chains, via, only)
			switch st.LegacyMode {
			case xrayconf.ModeAmneziaOwn:
				st.ActiveChain = via.ID
			case xrayconf.ModeOwn:
				st.ActiveChain = only.ID
			}
			if st.Options.LegacyFetchViaOwn {
				st.Options.FetchVia = own.ID
			}
		}
	}
	st.LegacyMode, st.LegacyActiveOwn, st.Options.LegacyFetchViaOwn = "", "", false
	st.Version = stateVersion
}

func save(dir string, box SecretBox, st *State) error {
	plain, err := json.Marshal(st)
	if err != nil {
		return err
	}
	sealed, err := box.Seal(plain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(sealed); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Chmod(0o600)
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, stateFile))
}
