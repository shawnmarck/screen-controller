package profiles

import "testing"

func TestMatchProfileByActiveOutputs(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Profiles: map[string]Profile{
			"dual": {
				Label: "Dual",
				Monitors: []string{
					"DP-1,3840x2160@60,0x0,1",
					"HDMI-A-1,3840x2160@60,2560x0,1",
				},
			},
			"single": {
				Label: "Single",
				Monitors: []string{
					"DP-1,3840x2160@60,0x0,1",
					"HDMI-A-1,disable",
				},
			},
		},
	}
	order := []string{"dual", "single"}
	if got := MatchProfileByActiveOutputs(cfg, order, []string{"DP-1", "HDMI-A-1"}); got != "dual" {
		t.Fatalf("dual case: got %q", got)
	}
	if got := MatchProfileByActiveOutputs(cfg, order, []string{"DP-1"}); got != "single" {
		t.Fatalf("single case: got %q", got)
	}
	if got := MatchProfileByActiveOutputs(cfg, order, []string{"DP-1", "eDP-1"}); got != "" {
		t.Fatalf("no match: got %q", got)
	}
}
