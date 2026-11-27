package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Traffic statistics: bytes sent and received per day, for the chain or single server selected at the time,
// and directly (rules sending traffic around the proxy). Counted from Xray's counters while the proxy runs,
// sampled with the status and when it stops or restarts; kept on the device only, a year long.

const (
	statsFile     = "stats.json"
	statsKeepDays = 366
	statsDirect   = "direct"
)

// directTraffic is implemented by instances that count direct traffic separately (runner.Instance, the
// desktop TUN service's).
type directTraffic interface {
	DirectTraffic() (up, down int64)
}

// StatsEntry is the traffic of one chain, server or "direct" on one day.
type StatsEntry struct {
	Key     string `json:"key"`  // "chain:<id>", "server:<id>" or "direct"
	Kind    string `json:"kind"` // chain, server, direct
	Name    string `json:"name"` // the current name; the one it had then when it was deleted
	Deleted bool   `json:"deleted,omitempty"`
	Up      int64  `json:"up"`
	Down    int64  `json:"down"`
}

// StatsDay is one day (local time, YYYY-MM-DD) of statistics.
type StatsDay struct {
	Date    string       `json:"date"`
	Entries []StatsEntry `json:"entries"`
}

type statsCounter struct {
	Name string `json:"name"`
	Up   int64  `json:"up"`
	Down int64  `json:"down"`
}

type statsStore struct {
	Days map[string]map[string]*statsCounter `json:"days"`
}

// statsRun is the running instance's counters at the last sample.
type statsRun struct {
	key, name            string
	up, down, dUp, dDown int64
	dirty                bool
	savedAt              time.Time
}

func (e *Engine) loadStatsLocked() {
	if e.stats != nil {
		return
	}
	e.stats = &statsStore{Days: map[string]map[string]*statsCounter{}}
	if b, err := os.ReadFile(filepath.Join(e.dir, statsFile)); err == nil {
		json.Unmarshal(b, e.stats)
		if e.stats.Days == nil {
			e.stats.Days = map[string]map[string]*statsCounter{}
		}
	}
}

func (e *Engine) saveStatsLocked() {
	cutoff := time.Now().AddDate(0, 0, -statsKeepDays).Format(time.DateOnly)
	for d := range e.stats.Days {
		if d < cutoff {
			delete(e.stats.Days, d)
		}
	}
	b, _ := json.Marshal(e.stats)
	tmp := filepath.Join(e.dir, statsFile+".tmp")
	if os.WriteFile(tmp, b, 0o600) == nil {
		os.Rename(tmp, filepath.Join(e.dir, statsFile))
	}
	e.statRun.dirty, e.statRun.savedAt = false, time.Now()
}

// statsKeyLocked names what the proxy is running through now.
func (e *Engine) statsKeyLocked() (key, name string) {
	if id := e.st.ActiveServer; id != "" {
		if s := e.serverLocked(id); s != nil {
			return "server:" + id, s.Name
		}
	}
	if c := e.activeChainLocked(); c != nil {
		return "chain:" + c.ID, e.chainNameLocked(c)
	}
	return "", ""
}

// statsStartLocked begins counting a new instance (its counters start at zero).
func (e *Engine) statsStartLocked() {
	e.statRun.key, e.statRun.name = e.statsKeyLocked()
	e.statRun.up, e.statRun.down, e.statRun.dUp, e.statRun.dDown = 0, 0, 0, 0
}

// sampleStatsLocked adds the traffic since the last sample to today; save forces writing the file (stop,
// restart), otherwise it is written at most once a minute.
func (e *Engine) sampleStatsLocked(save bool) {
	if e.inst != nil {
		up, down := e.inst.Traffic()
		var dUp, dDown int64
		if d, ok := e.inst.(directTraffic); ok {
			dUp, dDown = d.DirectTraffic()
		}
		delta := func(cur, last int64) int64 {
			if cur < last { // counters restarted
				return cur
			}
			return cur - last
		}
		r := &e.statRun
		in, out := delta(up, r.up), delta(down, r.down)
		dIn, dOut := delta(dUp, r.dUp), delta(dDown, r.dDown)
		r.up, r.down, r.dUp, r.dDown = up, down, dUp, dDown
		if in+out > 0 {
			e.loadStatsLocked()
			day := time.Now().Format(time.DateOnly)
			m := e.stats.Days[day]
			if m == nil {
				m = map[string]*statsCounter{}
				e.stats.Days[day] = m
			}
			add := func(key, name string, u, d int64) {
				if u <= 0 && d <= 0 || key == "" {
					return
				}
				c := m[key]
				if c == nil {
					c = &statsCounter{}
					m[key] = c
				}
				c.Name = name
				c.Up += max(u, 0)
				c.Down += max(d, 0)
			}
			add(statsDirect, "", min(dIn, in), min(dOut, out))
			add(r.key, r.name, in-min(dIn, in), out-min(dOut, out))
			r.dirty = true
		}
	}
	if e.statRun.dirty && (save || time.Since(e.statRun.savedAt) > time.Minute) {
		e.saveStatsLocked()
	}
}

// Stats returns the last days of statistics (today included), oldest first; days without traffic are
// included empty so charts have every day.
func (e *Engine) Stats(days int) []StatsDay {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sampleStatsLocked(false)
	e.loadStatsLocked()
	if days < 1 {
		days = 1
	}
	out := make([]StatsDay, 0, days)
	now := time.Now()
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format(time.DateOnly)
		d := StatsDay{Date: date, Entries: []StatsEntry{}}
		for key, c := range e.stats.Days[date] {
			en := StatsEntry{Key: key, Name: c.Name, Up: c.Up, Down: c.Down}
			switch {
			case key == statsDirect:
				en.Kind = "direct"
			case strings.HasPrefix(key, "chain:"):
				en.Kind = "chain"
				if ch := e.chainLocked(strings.TrimPrefix(key, "chain:")); ch != nil {
					en.Name = e.chainNameLocked(ch)
				} else {
					en.Deleted = true
				}
			case strings.HasPrefix(key, "server:"):
				en.Kind = "server"
				if s := e.serverLocked(strings.TrimPrefix(key, "server:")); s != nil {
					en.Name = s.Name
				} else {
					en.Deleted = true
				}
			}
			d.Entries = append(d.Entries, en)
		}
		sort.Slice(d.Entries, func(a, b int) bool {
			return d.Entries[a].Up+d.Entries[a].Down > d.Entries[b].Up+d.Entries[b].Down
		})
		out = append(out, d)
	}
	return out
}

// ResetStats deletes all statistics.
func (e *Engine) ResetStats() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats = &statsStore{Days: map[string]map[string]*statsCounter{}}
	e.statRun.dirty = false
	err := os.Remove(filepath.Join(e.dir, statsFile))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
