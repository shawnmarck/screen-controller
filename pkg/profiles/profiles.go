package profiles

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PrimaryMonitor string             `yaml:"primary_monitor"`
	SafeWorkspace  int                `yaml:"safe_workspace"`
	ProfileOrder   []string           `yaml:"profile_order"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Label    string   `yaml:"label"`
	Monitors []string `yaml:"monitors"`
}

// fileCfg is the on-disk shape: safe_workspace may be 0, distinguished from omitted via pointer.
type fileCfg struct {
	PrimaryMonitor string             `yaml:"primary_monitor"`
	SafeWorkspace  *int               `yaml:"safe_workspace"`
	ProfileOrder   []string           `yaml:"profile_order"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f fileCfg
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.PrimaryMonitor == "" {
		return nil, fmt.Errorf("profiles: primary_monitor is required")
	}
	if f.SafeWorkspace == nil {
		return nil, fmt.Errorf("profiles: safe_workspace is required")
	}
	if len(f.Profiles) == 0 {
		return nil, fmt.Errorf("profiles: no profiles defined")
	}
	cfg := &Config{
		PrimaryMonitor: f.PrimaryMonitor,
		SafeWorkspace:  *f.SafeWorkspace,
		ProfileOrder:   f.ProfileOrder,
		Profiles:       f.Profiles,
	}
	for id, p := range cfg.Profiles {
		if !profileReferencesOutput(p, f.PrimaryMonitor) {
			return nil, fmt.Errorf("profiles: profile %q must include a monitor line for primary_monitor %q", id, f.PrimaryMonitor)
		}
	}
	return cfg, nil
}

func profileReferencesOutput(p Profile, output string) bool {
	for _, line := range p.Monitors {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, _, err := parseMonitorLine(line)
		if err != nil {
			continue
		}
		if name == output {
			return true
		}
	}
	return false
}

// OrderedIDs returns profile keys in profile_order, then any remaining keys sorted.
func (c *Config) OrderedIDs() []string {
	seen := make(map[string]struct{})
	var out []string
	for _, id := range c.ProfileOrder {
		if _, ok := c.Profiles[id]; !ok {
			continue
		}
		out = append(out, id)
		seen[id] = struct{}{}
	}
	var rest []string
	for id := range c.Profiles {
		if _, ok := seen[id]; ok {
			continue
		}
		rest = append(rest, id)
	}
	sort.Strings(rest)
	out = append(out, rest...)
	return out
}

// ActiveOutputs returns connector names that stay active (not ",disable" lines).
func ActiveOutputs(monitors []string) ([]string, error) {
	var out []string
	for i, line := range monitors {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, disabled, err := parseMonitorLine(line)
		if err != nil {
			return nil, fmt.Errorf("profile monitor[%d]: %w", i, err)
		}
		if disabled {
			continue
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("profile has no active monitors")
	}
	return out, nil
}

// ReferencedOutputs returns distinct connector names mentioned in monitor lines (active or disabled).
func ReferencedOutputs(monitors []string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string
	for i, line := range monitors {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, _, err := parseMonitorLine(line)
		if err != nil {
			return nil, fmt.Errorf("profile monitor[%d]: %w", i, err)
		}
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out, nil
}

// ConnectedOutputsNotInProfile returns outputs that are connected now but not named in the profile's monitor lines.
func ConnectedOutputsNotInProfile(current []string, referenced []string) []string {
	want := make(map[string]struct{}, len(referenced))
	for _, n := range referenced {
		want[n] = struct{}{}
	}
	var extra []string
	for _, n := range current {
		if _, ok := want[n]; !ok {
			extra = append(extra, n)
		}
	}
	return extra
}

func parseMonitorLine(line string) (name string, disabled bool, err error) {
	comma := strings.IndexByte(line, ',')
	if comma == -1 {
		return "", false, fmt.Errorf("invalid monitor line (no comma): %q", line)
	}
	name = strings.TrimSpace(line[:comma])
	rest := strings.TrimSpace(line[comma+1:])
	if name == "" {
		return "", false, fmt.Errorf("empty output name")
	}
	if rest == "disable" {
		return name, true, nil
	}
	return name, false, nil
}
