package hypr

import (
	"reflect"
	"testing"
)

func TestLinesFromLive(t *testing.T) {
	t.Parallel()
	ms := []Monitor{
		{
			Name:                  "DP-1",
			Width:                 3840,
			Height:                2160,
			RefreshRate:           143.99899,
			X:                     0,
			Y:                     0,
			Scale:                 1.5,
			CurrentFormat:         "XRGB8888",
			ColorManagementPreset: "srgb",
			AvailableModes:        []string{"3840x2160@60.00Hz", "3840x2160@144.00Hz"},
		},
		{
			Name:     "HDMI-A-1",
			Disabled: true,
		},
	}
	got, err := LinesFromLive(ms, []string{"HDMI-A-1", "DP-2"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"DP-1,3840x2160@144,0x0,1.5,bitdepth,8,cm,srgb",
		"HDMI-A-1,disable",
		"DP-2,disable",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestLinesFromLiveEmpty(t *testing.T) {
	t.Parallel()
	if _, err := LinesFromLive(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestBitDepth10(t *testing.T) {
	t.Parallel()
	if bitDepth("XRGB2101010") != "10" {
		t.Fatal(bitDepth("XRGB2101010"))
	}
}
