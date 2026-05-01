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

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.PrimaryMonitor == "" {
		return nil, fmt.Errorf("profiles: primary_monitor is required")
	}
	if cfg.SafeWorkspace == 0 {
		return nil, fmt.Errorf("profiles: safe_workspace is required")
	}
	if len(cfg.Profiles) == 0 {
		return nil, fmt.Errorf("profiles: no profiles defined")
	}
	return &cfg, nil
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
