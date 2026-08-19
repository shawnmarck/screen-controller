package hypr

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// LinesFromLive builds Hyprland monitor= lines from the current session.
// alsoDisable names (from other profiles) that are not in ms become ",disable" lines
// so a saved single-monitor layout still turns the other output off.
func LinesFromLive(ms []Monitor, alsoDisable []string) ([]string, error) {
	seen := make(map[string]struct{})
	var lines []string
	for _, m := range ms {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
		if m.Disabled {
			lines = append(lines, name+",disable")
			continue
		}
		line, err := activeLine(m)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("no monitors reported by Hyprland")
	}
	var extra []string
	for _, name := range alsoDisable {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		extra = append(extra, name)
	}
	sort.Strings(extra)
	for _, name := range extra {
		lines = append(lines, name+",disable")
	}
	return lines, nil
}

func activeLine(m Monitor) (string, error) {
	if m.Width <= 0 || m.Height <= 0 {
		return "", fmt.Errorf("monitor %s: invalid mode %dx%d", m.Name, m.Width, m.Height)
	}
	mode := modeToken(m)
	pos := fmt.Sprintf("%dx%d", m.X, m.Y)
	scale := formatScale(m.Scale)
	parts := []string{m.Name, mode, pos, scale}
	if m.Transform != 0 {
		parts = append(parts, "transform", strconv.Itoa(m.Transform))
	}
	if m.VRR {
		parts = append(parts, "vrr", "1")
	}
	if bits := bitDepth(m.CurrentFormat); bits != "" {
		parts = append(parts, "bitdepth", bits)
	}
	if cm := strings.TrimSpace(m.ColorManagementPreset); cm != "" {
		parts = append(parts, "cm", cm)
	}
	return strings.Join(parts, ","), nil
}

func modeToken(m Monitor) string {
	rate := preferredRate(m)
	return fmt.Sprintf("%dx%d@%s", m.Width, m.Height, rate)
}

func preferredRate(m Monitor) string {
	prefix := fmt.Sprintf("%dx%d@", m.Width, m.Height)
	best := ""
	bestDelta := math.MaxFloat64
	for _, raw := range m.AvailableModes {
		raw = strings.TrimSpace(raw)
		raw = strings.TrimSuffix(raw, "Hz")
		if !strings.HasPrefix(raw, prefix) {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimPrefix(raw, prefix), 64)
		if err != nil {
			continue
		}
		delta := math.Abs(n - m.RefreshRate)
		if delta < bestDelta {
			bestDelta = delta
			if n == math.Trunc(n) {
				best = strconv.Itoa(int(n))
			} else {
				best = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(n, 'f', 2, 64), "0"), ".")
			}
		}
	}
	if best != "" && bestDelta < 1.5 {
		return best
	}
	return strconv.Itoa(int(math.Round(m.RefreshRate)))
}

func formatScale(scale float64) string {
	if scale <= 0 {
		return "1"
	}
	s := strconv.FormatFloat(scale, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "1"
	}
	return s
}

func bitDepth(format string) string {
	f := strings.ToUpper(format)
	switch {
	case strings.Contains(f, "2101010"), strings.Contains(f, "101010"), strings.Contains(f, "XB30"), strings.Contains(f, "AB30"):
		return "10"
	case f == "":
		return ""
	default:
		return "8"
	}
}
