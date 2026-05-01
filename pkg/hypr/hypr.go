package hypr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type monitor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
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

func monitors() ([]monitor, error) {
	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl monitors: %w", err)
	}
	var ms []monitor
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
	var cs []client
	if err := json.Unmarshal(out, &cs); err != nil {
		return nil, fmt.Errorf("parse clients: %w", err)
	}
	return cs, nil
}

// MonitorNames returns currently connected output names (hyprctl order).
func MonitorNames() ([]string, error) {
	ms, err := monitors()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ms))
	for i := range ms {
		names[i] = ms[i].Name
	}
	return names, nil
}

func monitorIDToName(ms []monitor) map[int]string {
	m := make(map[int]string)
	for _, mon := range ms {
		m[mon.ID] = mon.Name
	}
	return m
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
	ms, err := monitors()
	if err != nil {
		return err
	}
	idToName := monitorIDToName(ms)
	cs, err := clients()
	if err != nil {
		return err
	}
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
		if c.Class == "screen-layout-tui" || c.InitialClass == "screen-layout-tui" {
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
			return fmt.Errorf("move %s (%s): %w", addr, c.Class, err)
		}
	}
	return nil
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

// ApplyMonitors runs hyprctl keyword monitor for each line in order.
func ApplyMonitors(lines []string) error {
	var parts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts = append(parts, "keyword monitor "+line)
	}
	if len(parts) == 0 {
		return fmt.Errorf("no monitor lines to apply")
	}
	batch := strings.Join(parts, "; ")
	cmd := exec.Command("hyprctl", "--batch", batch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("hyprctl --batch: %w: %s", err, msg)
		}
		return fmt.Errorf("hyprctl --batch: %w", err)
	}
	return nil
}
