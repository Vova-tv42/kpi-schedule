// Package engine implements the schedule merging & enrichment logic described
// in docs/architecture/merging-engine.md: personal lessons (my.kpi.ua) are the
// base source of truth for enrollment; group lessons (Campus API) are the
// metadata source used to enrich them.
package engine

import (
	"strings"
	"unicode"
)

// NormalizeSubject lower-cases, unifies apostrophes, and collapses whitespace
// so subject names from the two sources can be compared. See
// docs/architecture/merging-engine.md §3, Step 2.
func NormalizeSubject(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.ReplaceAll(s, "ʼ", "'")
	s = strings.ReplaceAll(s, "`", "'")

	var b strings.Builder
	lastSpace := true // trims leading whitespace
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsPunct(r) && r != '\'' && r != '-' {
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

// NormalizeTag maps a free-form type label to the Campus API's tag vocabulary.
func NormalizeTag(typeLabel string) string {
	t := strings.ToLower(strings.TrimSpace(typeLabel))
	switch {
	case strings.HasPrefix(t, "лек"):
		return "lec"
	case strings.HasPrefix(t, "прак") || strings.HasPrefix(t, "сем"):
		return "prac"
	case strings.HasPrefix(t, "лаб"):
		return "lab"
	default:
		return ""
	}
}
