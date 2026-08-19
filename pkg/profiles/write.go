package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Save writes cfg atomically. Profile maps are emitted in OrderedIDs order.
func Save(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("profiles: nil config")
	}
	if err := validateForSave(cfg); err != nil {
		return err
	}
	body, err := marshalConfig(cfg)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".profiles-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func validateForSave(cfg *Config) error {
	if cfg.PrimaryMonitor == "" {
		return fmt.Errorf("profiles: primary_monitor is required")
	}
	if len(cfg.Profiles) == 0 {
		return fmt.Errorf("profiles: no profiles defined")
	}
	for id, p := range cfg.Profiles {
		if err := checkID(id); err != nil {
			return err
		}
		if !profileReferencesOutput(p, cfg.PrimaryMonitor) {
			return fmt.Errorf("profiles: profile %q must include a monitor line for primary_monitor %q", id, cfg.PrimaryMonitor)
		}
		if _, err := ActiveOutputs(p.Monitors); err != nil {
			return fmt.Errorf("profiles: profile %q: %w", id, err)
		}
	}
	return nil
}

func marshalConfig(cfg *Config) ([]byte, error) {
	var b strings.Builder
	b.WriteString("# screen-controller layout profiles. Edited by the CLI and TUI.\n")
	b.WriteString("primary_monitor: ")
	b.WriteString(yamlScalar(cfg.PrimaryMonitor))
	b.WriteByte('\n')
	b.WriteString("safe_workspace: ")
	fmt.Fprintf(&b, "%d\n", cfg.SafeWorkspace)
	ids := cfg.OrderedIDs()
	if len(ids) > 0 {
		b.WriteString("profile_order:\n")
		for _, id := range ids {
			b.WriteString("  - ")
			b.WriteString(yamlScalar(id))
			b.WriteByte('\n')
		}
	}
	b.WriteString("profiles:\n")
	for _, id := range ids {
		p := cfg.Profiles[id]
		b.WriteString("  ")
		b.WriteString(id)
		b.WriteString(":\n")
		b.WriteString("    label: ")
		b.WriteString(yamlScalar(p.Label))
		b.WriteByte('\n')
		b.WriteString("    monitors:\n")
		for _, line := range p.Monitors {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			b.WriteString("      - ")
			b.WriteString(yamlScalar(line))
			b.WriteByte('\n')
		}
	}
	return []byte(b.String()), nil
}

func yamlScalar(s string) string {
	out, err := yaml.Marshal(s)
	if err != nil {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return strings.TrimSpace(string(out))
}
