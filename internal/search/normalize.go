package search

import (
	"strings"
	"unicode"
)

// Normalize lowercases text, replaces ё with е, drops punctuation and trims
// leading zeros from numeric tokens (so "МДК.01.02" matches "мдк 1.2").
func Normalize(s string) string {
	return strings.Join(Tokens(s), " ")
}

// Tokens splits text into comparable tokens.
func Tokens(s string) []string {
	var (
		buf    []rune
		tokens []string
	)

	flush := func() {
		if len(buf) == 0 {
			return
		}
		token := string(buf)
		if len(token) > 1 && allDigits(token) {
			token = strings.TrimLeft(token, "0")
			if token == "" {
				token = "0"
			}
		}
		tokens = append(tokens, token)
		buf = buf[:0]
	}

	for _, r := range s {
		r = unicode.ToLower(r)
		if r == 'ё' {
			r = 'е'
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf = append(buf, r)
			continue
		}
		flush()
	}
	flush()

	return tokens
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
