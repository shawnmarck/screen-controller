package profiles

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var validID = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)

// ValidID reports whether id is a safe YAML/CLI profile key.
func ValidID(id string) bool {
	return validID.MatchString(id)
}

func checkID(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid profile id %q (use lowercase letters, digits, _ or -, start with a letter, max 48)", id)
	}
	return nil
}

// SlugID turns a label into a ValidID, or returns "" if nothing usable remains.
func SlugID(label string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(label) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '-' || r == '_':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		case unicode.IsSpace(r) || r == '—' || r == '-' || r == '/':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	s := strings.Trim(b.String(), "_")
	if s == "" {
		return ""
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "p_" + s
	}
	if len(s) > 48 {
		s = s[:48]
		s = strings.Trim(s, "_")
	}
	if !ValidID(s) {
		return ""
	}
	return s
}
