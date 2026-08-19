package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func sampleCfg() *Config {
	return &Config{
		PrimaryMonitor: "DP-1",
		SafeWorkspace:  1,
		ProfileOrder:   []string{"dual_sdr", "single_left_sdr"},
		Profiles: map[string]Profile{
			"dual_sdr": {
				Label:    "Dual SDR",
				Monitors: []string{"DP-1,3840x2160@144,0x0,1.5", "HDMI-A-1,3840x2160@144,2560x0,1.5"},
			},
			"single_left_sdr": {
				Label:    "Single left",
				Monitors: []string{"DP-1,3840x2160@144,0x0,1.5", "HDMI-A-1,disable"},
			},
		},
	}
}

func TestUpsertRenameDeleteRoundTrip(t *testing.T) {
	t.Parallel()
	cfg := sampleCfg()
	if err := cfg.Upsert("single_left_hdr", "Single HDR", []string{
		"DP-1,3840x2160@144,0x0,1.5,bitdepth,10,cm,hdr",
		"HDMI-A-1,disable",
	}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Relabel("dual_sdr", "Dual — SDR"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Rename("single_left_sdr", "single_sdr"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Profiles["single_left_sdr"]; ok {
		t.Fatal("old id still present")
	}
	if err := cfg.Delete("single_sdr"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Delete("single_left_hdr"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Delete("dual_sdr"); err == nil {
		t.Fatal("expected refuse last delete")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["dual_sdr"].Label != "Dual — SDR" {
		t.Fatalf("label=%q", got.Profiles["dual_sdr"].Label)
	}
	if _, ok := got.Profiles["single_left_hdr"]; ok {
		t.Fatal("deleted profile came back")
	}
}

func TestUpsertRejectsMissingPrimary(t *testing.T) {
	t.Parallel()
	cfg := sampleCfg()
	err := cfg.Upsert("bad", "Bad", []string{"HDMI-A-1,3840x2160@60,0x0,1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAllReferencedOutputs(t *testing.T) {
	t.Parallel()
	got := sampleCfg().AllReferencedOutputs()
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestSaveCreatesDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "profiles.yaml")
	if err := Save(path, sampleCfg()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
