package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awesome-gocui/gocui"

	"screen-controller/pkg/hypr"
	"screen-controller/pkg/profiles"
	"screen-controller/pkg/theme"
)

type app struct {
	cfg        *profiles.Config
	configPath string
	ids        []string
	cursor     int
	status     string
	col        theme.Colors
}

func main() {
	home, _ := os.UserHomeDir()
	defaultCfg := filepath.Join(home, "projects", "screen-controller", "profiles.yaml")
	configPath := flag.String("config", defaultCfg, "path to profiles.yaml")
	kittyPath := flag.String("theme-kitty", theme.DefaultKittyConf(), "kitty.conf for palette (Omarchy theme)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s [flags]                  # TUI\n  %s [flags] apply <profile_id> # one-shot layout\n  %s [flags] list               # profile ids\n", os.Args[0], os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) >= 1 {
		switch args[0] {
		case "apply":
			if len(args) < 2 {
				flag.Usage()
				os.Exit(2)
			}
			if err := runApply(*configPath, args[1]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "list":
			if err := runList(*configPath); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}

	cfg, err := profiles.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	a := &app{
		cfg:        cfg,
		configPath: *configPath,
		ids:        cfg.OrderedIDs(),
		status:     "↑/↓ j/k  Enter apply  r reload  q quit",
		col:        theme.FromKittyPath(*kittyPath),
	}
	if len(a.ids) == 0 {
		fmt.Fprintln(os.Stderr, "no profiles to show")
		os.Exit(1)
	}

	g, err := gocui.NewGui(gocui.OutputTrue, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gocui: %v\n", err)
		os.Exit(1)
	}
	defer g.Close()
	g.Cursor = true
	g.Highlight = true
	g.SelFgColor = a.col.Accent
	g.SelBgColor = a.col.BG
	g.FgColor = a.col.FG
	g.BgColor = a.col.BG

	g.SetManagerFunc(a.layout)

	bind := func(key interface{}, mod gocui.Modifier, fn func(*gocui.Gui, *gocui.View) error) {
		if err := g.SetKeybinding("", key, mod, fn); err != nil {
			fmt.Fprintf(os.Stderr, "keybinding: %v\n", err)
			os.Exit(1)
		}
	}
	bind(gocui.KeyCtrlC, gocui.ModNone, quit)
	bind('q', gocui.ModNone, quit)
	bind(gocui.KeyEsc, gocui.ModNone, quit)
	bind('j', gocui.ModNone, a.cursorDown)
	bind(gocui.KeyArrowDown, gocui.ModNone, a.cursorDown)
	bind('k', gocui.ModNone, a.cursorUp)
	bind(gocui.KeyArrowUp, gocui.ModNone, a.cursorUp)
	bind(gocui.KeyEnter, gocui.ModNone, a.applyCurrent)
	bind(10, gocui.ModNone, a.applyCurrent)
	bind('r', gocui.ModNone, a.reloadConfig)

	if err := g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		fmt.Fprintf(os.Stderr, "MainLoop: %v\n", err)
		os.Exit(1)
	}
}

func quit(g *gocui.Gui, _ *gocui.View) error {
	return gocui.ErrQuit
}

func (a *app) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if maxX < 20 || maxY < 8 {
		return nil
	}
	title := " Screen layouts "
	if v, err := g.SetView("title", 0, 0, maxX-1, 2, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Frame = false
		v.FgColor = a.col.Accent
		v.BgColor = a.col.BG
		fmt.Fprintln(v, title+strings.Repeat(" ", max(0, maxX-len(title)-2)))
	} else {
		v.Clear()
		v.FgColor = a.col.Accent
		fmt.Fprintln(v, title)
	}

	listTop, listBot := 2, maxY-3
	if v, err := g.SetView("list", 0, listTop, maxX-1, listBot, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Frame = true
		v.Title = " Profiles "
		v.FgColor = a.col.FG
		v.BgColor = a.col.BG
		v.TitleColor = a.col.Accent
		v.FrameColor = a.col.Border
		v.SelBgColor = a.col.BG
		v.SelFgColor = a.col.Accent
		v.Highlight = true
		v.Wrap = false
		v.Editable = false
		if _, err := g.SetCurrentView("list"); err != nil {
			return err
		}
	}
	v, _ := g.View("list")
	v.Clear()
	v.FgColor = a.col.FG
	for i, id := range a.ids {
		p := a.cfg.Profiles[id]
		marker := " "
		if i == a.cursor {
			marker = "▸"
		}
		fmt.Fprintf(v, "  %s %s\n", marker, p.Label)
	}
	_ = v.SetCursor(0, min(a.cursor, max(0, len(a.ids)-1)))

	if v, err := g.SetView("footer", 0, maxY-2, maxX-1, maxY, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Frame = false
		v.FgColor = a.col.Dim
		v.BgColor = a.col.BG
	} else {
		v.Clear()
		v.FgColor = a.col.Dim
	}
	v, _ = g.View("footer")
	v.Clear()
	fmt.Fprintln(v, a.status)

	_, _ = g.SetCurrentView("list")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *app) cursorDown(_ *gocui.Gui, _ *gocui.View) error {
	if a.cursor < len(a.ids)-1 {
		a.cursor++
	}
	return nil
}

func (a *app) cursorUp(_ *gocui.Gui, _ *gocui.View) error {
	if a.cursor > 0 {
		a.cursor--
	}
	return nil
}

func (a *app) reloadConfig(_ *gocui.Gui, _ *gocui.View) error {
	cfg, err := profiles.Load(a.configPath)
	if err != nil {
		a.status = "reload: " + err.Error()
		return nil
	}
	a.cfg = cfg
	a.ids = cfg.OrderedIDs()
	if a.cursor >= len(a.ids) {
		a.cursor = max(0, len(a.ids)-1)
	}
	a.status = "Reloaded profiles.yaml"
	return nil
}

func (a *app) applyCurrent(_ *gocui.Gui, _ *gocui.View) error {
	if len(a.ids) == 0 {
		return nil
	}
	id := a.ids[a.cursor]
	if err := applyProfile(a.cfg, id); err != nil {
		a.status = err.Error()
		return nil
	}
	a.status = "OK — " + a.cfg.Profiles[id].Label
	return nil
}

func runApply(configPath, profileID string) error {
	cfg, err := profiles.Load(configPath)
	if err != nil {
		return err
	}
	return applyProfile(cfg, profileID)
}

func runList(configPath string) error {
	cfg, err := profiles.Load(configPath)
	if err != nil {
		return err
	}
	for _, id := range cfg.OrderedIDs() {
		fmt.Printf("%s\t%s\n", id, cfg.Profiles[id].Label)
	}
	return nil
}

func applyProfile(cfg *profiles.Config, profileID string) error {
	p, ok := cfg.Profiles[profileID]
	if !ok {
		return fmt.Errorf("unknown profile %q (run: list)", profileID)
	}
	active, err := profiles.ActiveOutputs(p.Monitors)
	if err != nil {
		return err
	}
	current, err := hypr.MonitorNames()
	if err != nil {
		return err
	}
	rem := hypr.RemovingOutputs(current, active)
	if err := hypr.MigrateOffMonitors(rem, cfg.SafeWorkspace); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := hypr.ApplyMonitors(p.Monitors); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
}
