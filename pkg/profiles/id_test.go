package profiles

import "testing"

func TestValidID(t *testing.T) {
	t.Parallel()
	ok := []string{"dual_sdr", "a", "p_1", "single-left"}
	for _, id := range ok {
		if !ValidID(id) {
			t.Fatalf("expected valid: %q", id)
		}
	}
	bad := []string{"", "Dual", "1abc", "has space", "UPPER"}
	for _, id := range bad {
		if ValidID(id) {
			t.Fatalf("expected invalid: %q", id)
		}
	}
}

func TestSlugID(t *testing.T) {
	t.Parallel()
	if got := SlugID("Dual — SDR (both 4K144)"); got != "dual_sdr_both_4k144" {
		t.Fatalf("got %q", got)
	}
	if got := SlugID("9 lives"); got != "p_9_lives" {
		t.Fatalf("got %q", got)
	}
}
