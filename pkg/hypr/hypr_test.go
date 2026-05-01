package hypr

import (
	"reflect"
	"testing"
)

func TestRemovingOutputs(t *testing.T) {
	t.Parallel()
	got := RemovingOutputs([]string{"DP-1", "HDMI-A-1", "eDP-1"}, []string{"DP-1"})
	want := []string{"HDMI-A-1", "eDP-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if len(RemovingOutputs([]string{"DP-1"}, []string{"DP-1", "HDMI-A-1"})) != 0 {
		t.Fatal("expected no removals")
	}
}
