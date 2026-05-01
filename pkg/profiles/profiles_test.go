package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMonitorLine(t *testing.T) {
	t.Parallel()
	name, dis, err := parseMonitorLine("DP-1,3840x2160@60,0x0,1")
	if err != nil {
		t.Fatal(err)
	}
	if dis || name != "DP-1" {
		t.Fatalf("got name=%q disabled=%v", name, dis)
	}
	name, dis, err = parseMonitorLine("  HDMI-A-1 , disable ")
	if err != nil {
		t.Fatal(err)
	}
	if !dis || name != "HDMI-A-1" {
		t.Fatalf("got name=%q disabled=%v", name, dis)
	}
	_, _, err = parseMonitorLine("nocomma")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestActiveOutputs(t *testing.T) {
	t.Parallel()
	out, err := ActiveOutputs([]string{
		"DP-1,3840x2160@60,0x0,1",
		"HDMI-A-1,disable",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0] != "DP-1" {
		t.Fatalf("got %v", out)
	}
	_, err = ActiveOutputs([]string{"HDMI-A-1,disable"})
	if err == nil {
		t.Fatal("expected error for no active monitors")
	}
}

func TestReferencedOutputs(t *testing.T) {
	t.Parallel()
	out, err := ReferencedOutputs([]string{
		"DP-1,3840x2160@60,0x0,1",
		"HDMI-A-1,disable",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %v", out)
	}
}

func TestConnectedOutputsNotInProfile(t *testing.T) {
	t.Parallel()
	ref := []string{"DP-1", "HDMI-A-1"}
	extra := ConnectedOutputsNotInProfile([]string{"DP-1", "HDMI-A-1", "eDP-1"}, ref)
	if len(extra) != 1 || extra[0] != "eDP-1" {
		t.Fatalf("got %v", extra)
	}
}

func TestOrderedIDs(t *testing.T) {
	t.Parallel()
	c := &Config{
		ProfileOrder: []string{"b", "missing", "a"},
		Profiles: map[string]Profile{
			"a": {Label: "A"},
			"b": {Label: "B"},
			"c": {Label: "C"},
		},
	}
	ids := c.OrderedIDs()
	if len(ids) != 3 || ids[0] != "b" || ids[1] != "a" || ids[2] != "c" {
		t.Fatalf("got %v", ids)
	}
}

func TestLoad_safeWorkspaceZero(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "profiles.yaml")
	content := `primary_monitor: DP-1
safe_workspace: 0
profiles:
  one:
    label: One
    monitors:
      - DP-1,disable
      - HDMI-A-1,3840x2160@60,0x0,1
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SafeWorkspace != 0 {
		t.Fatalf("SafeWorkspace=%d", cfg.SafeWorkspace)
	}
}

func TestLoad_primaryMustAppearInEachProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "profiles.yaml")
	content := `primary_monitor: DP-1
safe_workspace: 1
profiles:
  bad:
    label: Bad
    monitors:
      - HDMI-A-1,3840x2160@60,0x0,1
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error: primary not in profile")
	}
}

func TestLoad_missingSafeWorkspace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "profiles.yaml")
	content := `primary_monitor: DP-1
profiles:
  one:
    label: One
    monitors:
      - DP-1,3840x2160@60,0x0,1
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error")
	}
}
