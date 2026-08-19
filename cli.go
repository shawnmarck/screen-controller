package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"screen-controller/pkg/hypr"
	"screen-controller/pkg/profiles"
)

type jsonState struct {
	OK             bool          `json:"ok"`
	Error          string        `json:"error,omitempty"`
	ConfigPath     string        `json:"configPath,omitempty"`
	PrimaryMonitor string        `json:"primaryMonitor,omitempty"`
	SafeWorkspace  int           `json:"safeWorkspace,omitempty"`
	MatchedID      string        `json:"matchedId,omitempty"`
	Hyprland       string        `json:"hyprland,omitempty"`
	Profiles       []jsonProfile `json:"profiles,omitempty"`
}

type jsonProfile struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Matched  bool     `json:"matched"`
	Monitors []string `json:"monitors"`
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func failJSON(err error) {
	writeJSON(jsonState{OK: false, Error: err.Error()})
}

func collectState(configPath string) (jsonState, error) {
	cfg, err := profiles.Load(configPath)
	if err != nil {
		return jsonState{}, err
	}
	st := jsonState{
		OK:             true,
		ConfigPath:     configPath,
		PrimaryMonitor: cfg.PrimaryMonitor,
		SafeWorkspace:  cfg.SafeWorkspace,
	}
	ids := cfg.OrderedIDs()
	var names []string
	if ms, err := hypr.Monitors(); err == nil {
		st.Hyprland = hypr.FormatMonitorsOneLine(ms, 0)
		names = make([]string, len(ms))
		for i := range ms {
			names[i] = ms[i].Name
		}
		st.MatchedID = profiles.MatchProfileByActiveOutputs(cfg, ids, names)
	}
	for _, id := range ids {
		st.Profiles = append(st.Profiles, jsonProfile{
			ID:       id,
			Label:    cfg.Profiles[id].Label,
			Matched:  id == st.MatchedID,
			Monitors: append([]string(nil), cfg.Profiles[id].Monitors...),
		})
	}
	return st, nil
}

func emitState(configPath string, jsonOut bool) error {
	if !jsonOut {
		return nil
	}
	st, err := collectState(configPath)
	if err != nil {
		return err
	}
	writeJSON(st)
	return nil
}

func runCLI(configPath string, jsonOut bool, labelFlag string, args []string) int {
	if len(args) == 0 {
		return -1
	}
	cmd := args[0]
	rest := args[1:]
	var err error
	switch cmd {
	case "list":
		if jsonOut {
			err = emitState(configPath, true)
		} else {
			err = runList(configPath)
		}
	case "describe":
		profileArg := ""
		if len(rest) >= 1 {
			profileArg = rest[0]
		}
		if jsonOut {
			err = emitState(configPath, true)
			_ = profileArg
		} else {
			if err = hypr.CheckSession(); err == nil {
				err = runDescribe(configPath, profileArg)
			}
		}
	case "apply":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: screen-controller apply <profile_id>")
			return 2
		}
		if err = hypr.CheckSession(); err == nil {
			err = runApply(configPath, rest[0])
		}
		if err == nil && jsonOut {
			err = emitState(configPath, true)
		}
	case "save":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: screen-controller save <profile_id> [--label NAME]")
			return 2
		}
		if err = hypr.CheckSession(); err == nil {
			err = runSave(configPath, rest[0], labelFlag)
		}
		if err == nil && jsonOut {
			err = emitState(configPath, true)
		} else if err == nil {
			fmt.Printf("saved %s\n", rest[0])
		}
	case "delete":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: screen-controller delete <profile_id>")
			return 2
		}
		err = runDelete(configPath, rest[0])
		if err == nil && jsonOut {
			err = emitState(configPath, true)
		} else if err == nil {
			fmt.Printf("deleted %s\n", rest[0])
		}
	case "rename":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "usage: screen-controller rename <old_id> <new_id>")
			return 2
		}
		err = runRename(configPath, rest[0], rest[1])
		if err == nil && jsonOut {
			err = emitState(configPath, true)
		} else if err == nil {
			fmt.Printf("renamed %s -> %s\n", rest[0], rest[1])
		}
	case "relabel":
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "usage: screen-controller relabel <profile_id> <label>")
			return 2
		}
		err = runRelabel(configPath, rest[0], strings.Join(rest[1:], " "))
		if err == nil && jsonOut {
			err = emitState(configPath, true)
		} else if err == nil {
			fmt.Printf("relabeled %s\n", rest[0])
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (expected list, apply, describe, save, delete, rename, relabel)\n\n", cmd)
		flag.Usage()
		return 2
	}
	if err != nil {
		if jsonOut {
			failJSON(err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	return 0
}

func runSave(configPath, id, label string) error {
	cfg, err := profiles.Load(configPath)
	if err != nil {
		return err
	}
	ms, err := hypr.Monitors()
	if err != nil {
		return err
	}
	lines, err := hypr.LinesFromLive(ms, cfg.AllReferencedOutputs())
	if err != nil {
		return err
	}
	if existing, ok := cfg.Profiles[id]; ok && strings.TrimSpace(label) == "" {
		label = existing.Label
	}
	if strings.TrimSpace(label) == "" {
		label = id
	}
	if err := cfg.Upsert(id, label, lines); err != nil {
		return err
	}
	return profiles.Save(configPath, cfg)
}

func runDelete(configPath, id string) error {
	cfg, err := profiles.Load(configPath)
	if err != nil {
		return err
	}
	if err := cfg.Delete(id); err != nil {
		return err
	}
	return profiles.Save(configPath, cfg)
}

func runRename(configPath, oldID, newID string) error {
	cfg, err := profiles.Load(configPath)
	if err != nil {
		return err
	}
	if err := cfg.Rename(oldID, newID); err != nil {
		return err
	}
	return profiles.Save(configPath, cfg)
}

func runRelabel(configPath, id, label string) error {
	cfg, err := profiles.Load(configPath)
	if err != nil {
		return err
	}
	if err := cfg.Relabel(id, label); err != nil {
		return err
	}
	return profiles.Save(configPath, cfg)
}
