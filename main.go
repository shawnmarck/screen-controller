package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/awesome-gocui/gocui"

	"screen-controller/pkg/hypr"
	"screen-controller/pkg/profiles"
	"screen-controller/pkg/theme"
)

const hyprPollInterval = 1500 * time.Millisecond

type app struct {
	cfg        *profiles.Config
	configPath string
	ids        []string
	cursor     int
	status     string
	col        theme.Colors

	showHelp bool

	hyprMonitors []hypr.Monitor
	matchedID    string
	liveLine     string
	hyprCacheAt  time.Time
	hyprForce    bool
}

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to profiles.yaml")
	kittyPath := flag.String("theme-kitty", theme.DefaultKittyConf(), "kitty.conf for palette (Omarchy theme)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s [flags]                  # TUI\n  %s [flags] apply <profile_id> # one-shot layout\n  %s [flags] list               # profile ids\n  %s [flags] describe [profile_id] # Hyprland vs profiles (optional id)\n", os.Args[0], os.Args[0], os.Args[0], os.Args[0])
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
			if err := hypr.CheckSession(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
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
		case "describe":
			if err := hypr.CheckSession(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			profileArg := ""
			if len(args) >= 2 {
				profileArg = args[1]
			}
			if err := runDescribe(*configPath, profileArg); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q (expected list, apply, or describe)\n\n", args[0])
			flag.Usage()
			os.Exit(2)
		}
	}

	if err := hypr.CheckSession(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
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
		status:     "Ready",
		col:        theme.FromKittyPath(*kittyPath),
		hyprForce:  true,
	}
	if len(a.ids) == 0 {
		fmt.Fprintln(os.Stderr, "no profiles to show")
		os.Exit(1)
	}

	g, err := newGocui()
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
	bind(gocui.KeyCtrlC, gocui.ModNone, a.quitOrCloseHelp)
	bind('q', gocui.ModNone, a.quitOrCloseHelp)
	bind(gocui.KeyEsc, gocui.ModNone, a.quitOrCloseHelp)
	bind('j', gocui.ModNone, a.cursorDown)
	bind(gocui.KeyArrowDown, gocui.ModNone, a.cursorDown)
	bind('k', gocui.ModNone, a.cursorUp)
	bind(gocui.KeyArrowUp, gocui.ModNone, a.cursorUp)
	bind(gocui.KeyEnter, gocui.ModNone, a.applyCurrent)
	bind('r', gocui.ModNone, a.reloadConfig)
	bind(gocui.KeyHome, gocui.ModNone, a.cursorHome)
	bind(gocui.KeyEnd, gocui.ModNone, a.cursorEnd)
	bind('?', gocui.ModNone, a.toggleHelp)
	for ch := '1'; ch <= '9'; ch++ {
		d := ch
		bind(d, gocui.ModNone, a.jumpDigit(d))
	}

	if err := g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		fmt.Fprintf(os.Stderr, "MainLoop: %v\n", err)
		os.Exit(1)
	}
}

// defaultConfigPath prefers XDG config, then a legacy repo-style path if that file exists, else XDG path for new installs.
func defaultConfigPath() string {
	cfgDir, err := os.UserConfigDir()
	if err == nil {
		xdg := filepath.Join(cfgDir, "screen-controller", "profiles.yaml")
		if _, e := os.Stat(xdg); e == nil {
			return xdg
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		legacy := filepath.Join(home, "projects", "screen-controller", "profiles.yaml")
		if _, e := os.Stat(legacy); e == nil {
			return legacy
		}
		if cfgDir != "" {
			return filepath.Join(cfgDir, "screen-controller", "profiles.yaml")
		}
		return legacy
	}
	if cfgDir != "" {
		return filepath.Join(cfgDir, "screen-controller", "profiles.yaml")
	}
	return filepath.Join(".", "profiles.yaml")
}

func newGocui() (*gocui.Gui, error) {
	modes := []gocui.OutputMode{
		gocui.OutputTrue,
		gocui.OutputNormal,
		gocui.Output256,
	}
	var lastErr error
	for _, mode := range modes {
		g, err := gocui.NewGui(mode, true)
		if err == nil {
			return g, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (a *app) quitOrCloseHelp(g *gocui.Gui, _ *gocui.View) error {
	if a.showHelp {
		a.showHelp = false
		_ = g.DeleteView("help")
		return nil
	}
	return gocui.ErrQuit
}

func (a *app) toggleHelp(_ *gocui.Gui, _ *gocui.View) error {
	a.showHelp = !a.showHelp
	return nil
}

func (a *app) jumpDigit(ch rune) func(*gocui.Gui, *gocui.View) error {
	return func(_ *gocui.Gui, _ *gocui.View) error {
		if a.showHelp {
			return nil
		}
		i := int(ch - '1')
		if i < 0 || i >= len(a.ids) {
			return nil
		}
		a.cursor = i
		return nil
	}
}

func (a *app) cursorHome(_ *gocui.Gui, _ *gocui.View) error {
	if !a.showHelp {
		a.cursor = 0
	}
	return nil
}

func (a *app) cursorEnd(_ *gocui.Gui, _ *gocui.View) error {
	if !a.showHelp && len(a.ids) > 0 {
		a.cursor = len(a.ids) - 1
	}
	return nil
}

func (a *app) refreshHypr(force bool) {
	if !force && !a.hyprCacheAt.IsZero() && time.Since(a.hyprCacheAt) < hyprPollInterval {
		return
	}
	ms, err := hypr.Monitors()
	a.hyprCacheAt = time.Now()
	if err != nil {
		a.hyprMonitors = nil
		a.liveLine = "hyprctl monitors: " + err.Error()
		a.matchedID = ""
		return
	}
	a.hyprMonitors = ms
	names := make([]string, len(ms))
	for i := range ms {
		names[i] = ms[i].Name
	}
	a.matchedID = profiles.MatchProfileByActiveOutputs(a.cfg, a.ids, names)
	a.liveLine = hypr.FormatMonitorsOneLine(ms, 0)
}

const helpScreen = `Keybindings
  j / k / arrows    move selection
  Home / End        first / last profile
  1–9               jump to profile by position
  Enter             apply selected profile
  r                 reload profiles.yaml
  ?                 toggle this help
  q / Esc / Ctrl+C  close help, or quit

Matching
  A profile is marked active (●) when Hyprland’s connected
  outputs exactly match that profile’s enabled outputs.

`

func (a *app) drawHelp(g *gocui.Gui, maxX, maxY int) error {
	v, err := g.SetView("help", 0, 0, maxX-1, maxY-1, 0)
	if err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = " Help "
		v.Frame = true
		v.Wrap = true
		v.FgColor = a.col.FG
		v.BgColor = a.col.BG
		v.TitleColor = a.col.Accent
		v.FrameColor = a.col.Border
	}
	v.Clear()
	fmt.Fprint(v, helpScreen)
	fmt.Fprintf(v, "\nConfig: %s\n", a.configPath)
	_, _ = g.SetCurrentView("help")
	return nil
}

func (a *app) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if maxX < 20 || maxY < 10 {
		return nil
	}
	if a.showHelp {
		return a.drawHelp(g, maxX, maxY)
	}
	_ = g.DeleteView("help")

	a.refreshHypr(a.hyprForce)
	a.hyprForce = false

	title := " Screen layouts "
	if v, err := g.SetView("title", 0, 0, maxX-1, 1, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Frame = false
		v.FgColor = a.col.Accent
		v.BgColor = a.col.BG
		fmt.Fprintln(v, title+strings.Repeat(" ", max(0, maxX-utf8.RuneCountInString(title)-1)))
	} else {
		v.Clear()
		v.FgColor = a.col.Accent
		fmt.Fprintln(v, title)
	}

	liveLine := a.liveLine
	if len(a.hyprMonitors) > 0 {
		liveLine = hypr.FormatMonitorsOneLine(a.hyprMonitors, max(12, maxX-12))
	}
	if v, err := g.SetView("live", 0, 2, maxX-1, 2, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Frame = false
		v.FgColor = a.col.Dim
		v.BgColor = a.col.BG
		v.Wrap = false
	} else {
		v.Clear()
		v.FgColor = a.col.Dim
	}
	vLive, _ := g.View("live")
	vLive.Clear()
	fmt.Fprintln(vLive, "Hyprland: "+liveLine)

	wide := maxX >= 86
	listRight := maxX - 1
	detailX0 := 0
	if wide {
		detailX0 = max(44, maxX/2)
		listRight = detailX0 - 1
	}

	listTop, listBot := 3, maxY-4
	if v, err := g.SetView("list", 0, listTop, listRight, listBot, 0); err != nil {
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
		activeMark := " "
		if id == a.matchedID {
			activeMark = "●"
		}
		cursorMark := " "
		if i == a.cursor {
			cursorMark = "▸"
		}
		fmt.Fprintf(v, " %s%s %s\n", activeMark, cursorMark, p.Label)
	}
	_ = v.SetCursor(0, min(a.cursor, max(0, len(a.ids)-1)))

	if wide {
		if v, err := g.SetView("detail", detailX0, listTop, maxX-1, listBot, 0); err != nil {
			if !errors.Is(err, gocui.ErrUnknownView) {
				return err
			}
			v.Frame = true
			v.Title = " monitor= lines "
			v.FgColor = a.col.FG
			v.BgColor = a.col.BG
			v.TitleColor = a.col.Accent
			v.FrameColor = a.col.Border
			v.Wrap = true
		}
		vd, _ := g.View("detail")
		vd.Clear()
		if len(a.ids) > 0 && a.cursor < len(a.ids) {
			pid := a.ids[a.cursor]
			dw := max(8, maxX-detailX0-2)
			for _, line := range a.cfg.Profiles[pid].Monitors {
				fmt.Fprintln(vd, truncateRunes(strings.TrimSpace(line), dw))
			}
		}
	} else {
		_ = g.DeleteView("detail")
	}

	matchLine := "Match: (none)"
	if a.matchedID != "" {
		mp := a.cfg.Profiles[a.matchedID]
		matchLine = fmt.Sprintf("Match: %s — %s", a.matchedID, mp.Label)
	}
	matchLine = truncateRunes(matchLine, maxX-2)

	if v, err := g.SetView("footer", 0, maxY-3, maxX-1, maxY-1, 0); err != nil {
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
	vf, _ := g.View("footer")
	vf.Clear()
	fmt.Fprintln(vf, matchLine)
	fmt.Fprintln(vf, truncateRunes(a.status, maxX-2))
	fmt.Fprint(vf, "↑↓j/k Home End 1-9 Enter  r  ?  q")

	_, _ = g.SetCurrentView("list")
	return nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
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
	if a.showHelp {
		return nil
	}
	if a.cursor < len(a.ids)-1 {
		a.cursor++
	}
	return nil
}

func (a *app) cursorUp(_ *gocui.Gui, _ *gocui.View) error {
	if a.showHelp {
		return nil
	}
	if a.cursor > 0 {
		a.cursor--
	}
	return nil
}

func (a *app) reloadConfig(_ *gocui.Gui, _ *gocui.View) error {
	if a.showHelp {
		return nil
	}
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
	if len(a.ids) == 0 {
		a.status = "Reloaded — no profiles left in YAML"
		return nil
	}
	a.status = "Reloaded profiles.yaml"
	a.hyprForce = true
	return nil
}

func (a *app) applyCurrent(_ *gocui.Gui, _ *gocui.View) error {
	if a.showHelp || len(a.ids) == 0 {
		return nil
	}
	id := a.ids[a.cursor]
	if err := applyProfile(a.cfg, id); err != nil {
		a.status = err.Error()
		return nil
	}
	a.status = "OK — " + a.cfg.Profiles[id].Label
	a.hyprForce = true
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

func runDescribe(configPath, profileID string) error {
	cfg, err := profiles.Load(configPath)
	if err != nil {
		return err
	}
	ids := cfg.OrderedIDs()
	ms, err := hypr.Monitors()
	if err != nil {
		return err
	}
	names := make([]string, len(ms))
	for i := range ms {
		names[i] = ms[i].Name
	}
	fmt.Println("Hyprland monitors (hyprctl monitors -j):")
	fmt.Println(hypr.FormatMonitorsOneLine(ms, 120))
	fmt.Println()
	matched := profiles.MatchProfileByActiveOutputs(cfg, ids, names)
	fmt.Printf("Matched profile id: %q\n", matched)
	fmt.Println("Profiles (enabled outputs vs current):")
	for _, id := range ids {
		act, err := profiles.ActiveOutputs(cfg.Profiles[id].Monitors)
		if err != nil {
			fmt.Printf("  %s: error: %v\n", id, err)
			continue
		}
		fmt.Printf("  %-22s active=%v  %s\n", id, act, describeMatchTag(id == matched))
	}
	if profileID != "" {
		p, ok := cfg.Profiles[profileID]
		if !ok {
			return fmt.Errorf("unknown profile %q (run: list)", profileID)
		}
		fmt.Println()
		fmt.Printf("Profile %q — %s\n", profileID, p.Label)
		for _, ln := range p.Monitors {
			fmt.Println("  " + strings.TrimSpace(ln))
		}
	}
	return nil
}

func describeMatchTag(match bool) string {
	if match {
		return "<- match"
	}
	return ""
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
	ref, err := profiles.ReferencedOutputs(p.Monitors)
	if err != nil {
		return err
	}
	if extra := profiles.ConnectedOutputsNotInProfile(current, ref); len(extra) > 0 {
		sort.Strings(extra)
		fmt.Fprintf(os.Stderr, "screen-controller: warning: connected outputs not listed in this profile (their windows were not migrated): %s\n", strings.Join(extra, ", "))
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
