package routing

func on() *bool { t := true; return &t }

// Preset is a named rule list the user can apply (replace or append).
type Preset struct {
	ID    string
	Rules []Rule
}

// Presets, modelled on v2rayNG's custom_routing_* assets.
var Presets = []Preset{
	{ID: "global", Rules: nil},
	{ID: "bypass_ru", Rules: []Rule{
		{ID: "ru-domains", Remarks: "Russian sites directly", Domain: []string{"geosite:category-ru"}, OutboundTag: OutDirect},
		{ID: "ru-ip", Remarks: "Russian IP addresses directly", IP: []string{"geoip:ru"}, OutboundTag: OutDirect},
	}},
	{ID: "block_ads", Rules: []Rule{
		{ID: "ads", Remarks: "Block ads", Domain: []string{"geosite:category-ads-all"}, OutboundTag: OutBlock},
	}},
	{ID: "bypass_bittorrent", Rules: []Rule{
		{ID: "bittorrent", Remarks: "BitTorrent directly", Protocol: []string{"bittorrent"}, OutboundTag: OutDirect},
	}},
	{ID: "block_quic", Rules: []Rule{
		{ID: "quic", Remarks: "Block QUIC (UDP 443)", Network: "udp", Port: "443", OutboundTag: OutBlock},
	}},
}

// PresetByID returns a copy of the preset's rules (nil, false if unknown).
func PresetByID(id string) ([]Rule, bool) {
	for _, p := range Presets {
		if p.ID == id {
			out := make([]Rule, len(p.Rules))
			copy(out, p.Rules)
			for i := range out {
				out[i].Enabled = on()
			}
			return out, true
		}
	}
	return nil, false
}

// Apply replaces or appends a preset's rules (rules with the same ID are not duplicated).
func (s Settings) Apply(presetID string, replace bool) (Settings, bool) {
	rules, ok := PresetByID(presetID)
	if !ok {
		return s, false
	}
	if replace {
		s.Rules = rules
		return s, true
	}
	have := map[string]bool{}
	for _, r := range s.Rules {
		have[r.ID] = true
	}
	for _, r := range rules {
		if !have[r.ID] {
			s.Rules = append(s.Rules, r)
		}
	}
	return s, true
}
