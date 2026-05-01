package profiles

import (
	"slices"
	"sort"
)

// MatchProfileByActiveOutputs returns the first profile id in order whose enabled outputs
// exactly match hyprMonitorNames (same multiset: sort-compare). Extra Hyprland outputs
// mean no profile matches until unplugged or a profile lists them.
func MatchProfileByActiveOutputs(c *Config, order []string, hyprMonitorNames []string) string {
	cur := append([]string(nil), hyprMonitorNames...)
	sort.Strings(cur)
	for _, id := range order {
		p, ok := c.Profiles[id]
		if !ok {
			continue
		}
		act, err := ActiveOutputs(p.Monitors)
		if err != nil {
			continue
		}
		want := append([]string(nil), act...)
		sort.Strings(want)
		if slices.Equal(want, cur) {
			return id
		}
	}
	return ""
}
