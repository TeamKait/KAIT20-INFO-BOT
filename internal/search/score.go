package search

import "strings"

// matchScore compares a normalized query with a normalized candidate name.
// It rewards full token coverage of the query and penalizes candidates that
// contain many unrelated tokens (so "Медведев" prefers the exact person over
// a longer unrelated name containing the same word).
func matchScore(query, candidate string) float64 {
	q := Tokens(query)
	c := Tokens(candidate)
	if len(q) == 0 || len(c) == 0 {
		return 0
	}

	covered := 0.0
	for _, qt := range q {
		best := 0.0
		for _, ct := range c {
			if s := tokenScore(qt, ct); s > best {
				best = s
			}
		}
		covered += best
	}

	hits := 0
	for _, ct := range c {
		best := 0.0
		for _, qt := range q {
			if s := tokenScore(qt, ct); s > best {
				best = s
			}
		}
		if best >= 0.85 {
			hits++
		}
	}

	coverage := covered / float64(len(q))
	candidateCoverage := float64(hits) / float64(len(c))
	return 0.8*coverage + 0.2*candidateCoverage
}

// tokenScore returns similarity of one token pair in [0, 1].
func tokenScore(query, candidate string) float64 {
	if query == candidate {
		return 1
	}
	// Prefixes cover "Медведев" -> "Медведев Владислав Сергеевич" and
	// initials like "в с" -> "Владислав Сергеевич".
	if strings.HasPrefix(candidate, query) || strings.HasPrefix(query, candidate) {
		return 0.95
	}

	maxLen := len([]rune(query))
	if n := len([]rune(candidate)); n > maxLen {
		maxLen = n
	}
	distance := Levenshtein(query, candidate)
	ratio := 1 - float64(distance)/float64(maxLen)

	if distance <= 2 && ratio >= 0.6 {
		return ratio
	}
	if ratio >= 0.85 {
		return ratio
	}
	return 0
}

func containsAll(normalized string, tokens []string) bool {
	for _, token := range tokens {
		if len([]rune(token)) < 2 {
			return false
		}
		if !strings.Contains(normalized, token) {
			return false
		}
	}
	return true
}
