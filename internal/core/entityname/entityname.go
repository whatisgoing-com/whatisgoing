// Package entityname normalizes raw NER-extracted entity text before it
// becomes entity identity (issue #26) — cheap, dependency-free cleanup
// that catches one class of duplicate ("Donald Trump's" vs "Donald
// Trump") outright, on top of whatever entity resolution (see
// cmd/entity-resolver) catches later.
package entityname

import "strings"

// possessiveSuffixes is checked longest-first so "Trump's" strips to
// "Trump" in one pass rather than leaving a dangling apostrophe.
var possessiveSuffixes = []string{"'s", "’s", "'", "’"}

// Normalize strips a trailing possessive marker and surrounding
// whitespace from name. It does not otherwise change casing, word order,
// or abbreviate/expand anything — this is deliberately the cheapest fix
// that catches real observed noise, not general text normalization.
func Normalize(name string) string {
	name = strings.TrimSpace(name)
	for _, suffix := range possessiveSuffixes {
		if trimmed, ok := strings.CutSuffix(name, suffix); ok {
			return strings.TrimSpace(trimmed)
		}
	}
	return name
}
