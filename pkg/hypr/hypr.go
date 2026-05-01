package hypr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Monitor is a subset of hyprctl monitors -j fields used for display and matching.
type Monitor struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	RefreshRate float64 `json:"refreshRate"`
}

type client struct {
	Address      string `json:"address"`
	Monitor      int    `json:"monitor"`
	Mapped       bool   `json:"mapped"`
	Hidden       bool   `json:"hidden"`
	Fullscreen   int    `json:"fullscreen"`
	Class        string `json:"class"`
	InitialClass string `json:"initialClass"`
	Title        string `json:"title"`
}

// ErrNotHyprland is returned when hyprctl is missing or not running under Hyprland.
var ErrNotHyprland = errors.New("not a Hyprland session (hyprctl unavailable or failed)")

// CheckSession verifies hyprctl works by running hyprctl version (avoids trusting a stale HYPRLAND_INSTANCE_SIGNATURE alone).
func CheckSession() error {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return fmt.Errorf("%w: hyprctl not in PATH", ErrNotHyprland)
	}
	out, err := exec.Command("hyprctl", "version").CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			return fmt.Errorf("%w: %s", ErrNotHyprland, s)
		}
		return fmt.Errorf("%w: %w", ErrNotHyprland, err)
	}
	return nil
}

// Monitors returns parsed hyprctl monitors -j entries (connected outputs Hyprland reports).
func Monitors() ([]Monitor, error) {
	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl monitors: %w", err)
	}
	out = bytes.TrimSpace(out)
	var ms []Monitor
	if err := json.Unmarshal(out, &ms); err != nil {
		return nil, fmt.Errorf("parse monitors: %w", err)
	}
	return ms, nil
}

func clients() ([]client, error) {
	out, err := exec.Command("hyprctl", "clients", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl clients: %w", err)
	}
	out = bytes.TrimSpace(out)
	var cs []client
	if err := json.Unmarshal(out, &cs); err != nil {
		return nil, fmt.Errorf("parse clients: %w", err)
	}
	return cs, nil
}

// MonitorNames returns currently connected output names (hyprctl order).
func MonitorNames() ([]string, error) {
	ms, err := Monitors()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ms))
	for i := range ms {
		names[i] = ms[i].Name
	}
	return names, nil
}

func monitorIDToName(ms []Monitor) map[int]string {
	m := make(map[int]string)
	for _, mon := range ms {
		m[mon.ID] = mon.Name
	}
	return m
}

// FormatMonitorsOneLine builds a compact summary for status lines (truncated to maxRunes runes if > 0).
func FormatMonitorsOneLine(ms []Monitor, maxRunes int) string {
	if len(ms) == 0 {
		return "(no monitors)"
	}
	var parts []string
	for _, m := range ms {
		parts = append(parts, fmt.Sprintf("%s %dx%d@%.0fHz", m.Name, m.Width, m.Height, m.RefreshRate))
	}
	s := strings.Join(parts, "  ·  ")
	if maxRunes > 0 && len([]rune(s)) > maxRunes {
		r := []rune(s)
		if maxRunes <= 1 {
			return "…"
		}
		s = string(r[:maxRunes-1]) + "…"
	}
	return s
}

// RemovingOutputs lists names present now but not in targetActive.
func RemovingOutputs(current []string, targetActive []string) []string {
	want := make(map[string]struct{})
	for _, n := range targetActive {
		want[n] = struct{}{}
	}
	var rm []string
	for _, n := range current {
		if _, ok := want[n]; !ok {
			rm = append(rm, n)
		}
	}
	return rm
}

// MigrateOffMonitors moves windows from outputs in `removing` to safeWorkspace.
func MigrateOffMonitors(removing []string, safeWorkspace int) error {
	if len(removing) == 0 {
		return nil
	}
	remove := make(map[string]struct{})
	for _, n := range removing {
		remove[n] = struct{}{}
	}
	ms, err := Monitors()
	if err != nil {
		return err
	}
	idToName := monitorIDToName(ms)
	cs, err := clients()
	if err != nil {
		return err
	}
	var moveErrs []string
	for _, c := range cs {
		if !c.Mapped || c.Hidden {
			continue
		}
		name, ok := idToName[c.Monitor]
		if !ok {
			continue
		}
		if _, bad := remove[name]; !bad {
			continue
		}
		if shouldSkipMigrate(c) {
			continue
		}
		addr := c.Address
		if addr == "" {
			continue
		}
		if c.Fullscreen != 0 {
			_ = dispatch("fullscreen", fmt.Sprintf("address:%s", addr))
		}
		arg := fmt.Sprintf("%d,address:%s", safeWorkspace, addr)
		if err := dispatch("movetoworkspacesilent", arg); err != nil {
			moveErrs = append(moveErrs, fmt.Sprintf("%s (%s): %v", addr, c.Class, err))
		}
	}
	if len(moveErrs) > 0 {
		const maxShow = 5
		show := moveErrs
		if len(show) > maxShow {
			show = append(show[:maxShow], fmt.Sprintf("… and %d more", len(moveErrs)-maxShow))
		}
		return fmt.Errorf("movetoworkspacesilent: %s", strings.Join(show, "; "))
	}
	return nil
}

func shouldSkipMigrate(c client) bool {
	switch {
	case c.Class == "screen-layout-tui", c.InitialClass == "screen-layout-tui":
		return true
	case c.Class == "org.omarchy.screen-controller", c.InitialClass == "org.omarchy.screen-controller":
		return true
	default:
		return false
	}
}

func dispatch(cmd string, args ...string) error {
	a := append([]string{"dispatch", cmd}, args...)
	out, err := exec.Command("hyprctl", a...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s != "" {
			return fmt.Errorf("%w: %s", err, s)
		}
		return err
	}
	return nil
}

// ApplyMonitors runs hyprctl keyword monitor for each line. It tries one --batch call,
// then falls back to one hyprctl invocation per line so a single bad token is easy to spot.
func ApplyMonitors(lines []string) error {
	var nonEmpty []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty = append(nonEmpty, line)
	}
	if len(nonEmpty) == 0 {
		return fmt.Errorf("no monitor lines to apply")
	}
	var parts []string
	for _, line := range nonEmpty {
		parts = append(parts, "keyword monitor "+line)
	}
	batch := strings.Join(parts, "; ")
	cmd := exec.Command("hyprctl", "--batch", batch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	errBatch := cmd.Run()
	if errBatch == nil {
		return nil
	}
	batchErr := fmt.Errorf("hyprctl --batch: %w: %s", errBatch, strings.TrimSpace(stderr.String()))

	var lineErrs []string
	allOK := true
	for _, line := range nonEmpty {
		c2 := exec.Command("hyprctl", "keyword", "monitor", line)
		var eout bytes.Buffer
		c2.Stderr = &eout
		if err := c2.Run(); err != nil {
			allOK = false
			msg := strings.TrimSpace(eout.String())
			if msg != "" {
				lineErrs = append(lineErrs, fmt.Sprintf("%q: %s", line, msg))
			} else {
				lineErrs = append(lineErrs, fmt.Sprintf("%q: %v", line, err))
			}
		}
	}
	if allOK {
		return nil
	}
	return fmt.Errorf("%v; per-line: %s", batchErr, strings.Join(lineErrs, "; "))
}
