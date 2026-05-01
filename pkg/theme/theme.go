package theme

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/awesome-gocui/gocui"
)

type Colors struct {
	FG     gocui.Attribute
	BG     gocui.Attribute
	Accent gocui.Attribute
	Border gocui.Attribute
	Dim    gocui.Attribute
}

// FromKittyPath parses Omarchy kitty.conf hex lines and maps via gocui.GetColor (truecolor when supported).
func FromKittyPath(path string) Colors {
	f, err := os.Open(path)
	if err != nil {
		return fallback()
	}
	defer f.Close()

	var fg, bg, c1, c8 string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], "=")
		val := strings.TrimPrefix(fields[1], "=")
		switch key {
		case "foreground":
			fg = val
		case "background":
			bg = val
		case "color1":
			c1 = val
		case "color8":
			c8 = val
		}
	}
	return Colors{
		FG:     mustColor(fg, "#ddf7ff"),
		BG:     mustColor(bg, "#0B0C16"),
		Accent: mustColor(c1, "#50f872"),
		Border: mustColor(c1, "#50f872"),
		Dim:    mustColor(c8, "#6a6e95"),
	}
}

func mustColor(s, def string) gocui.Attribute {
	s = strings.TrimSpace(s)
	if s == "" {
		s = def
	}
	c := gocui.GetColor(s)
	if c == 0 {
		c = gocui.GetColor(def)
	}
	return c
}

func fallback() Colors {
	return Colors{
		FG:     mustColor("", "#ddf7ff"),
		BG:     mustColor("", "#0B0C16"),
		Accent: mustColor("", "#50f872"),
		Border: mustColor("", "#50f872"),
		Dim:    mustColor("", "#6a6e95"),
	}
}

func DefaultKittyConf() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s/.config/omarchy/current/theme/kitty.conf", home)
}
