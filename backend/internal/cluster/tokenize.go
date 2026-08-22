package cluster

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Vector is a sparse weighted bag of tokens.
type Vector map[string]float64

// TokenTally is persisted thumbs feedback for one token.
type TokenTally struct {
	Up   int
	Down int
}

const (
	properNounWeight = 3
	plainWeight      = 1
)

func Tokenize(title, body string, weights map[string]TokenTally) Vector {
	stops := stopwordSet()
	vec := Vector{}
	addTokens(vec, title, false, stops, weights)
	addTokens(vec, body, true, stops, weights)
	return vec
}

func addTokens(vec Vector, raw string, body bool, stops map[string]bool, weights map[string]TokenTally) {
	plain := stripTags(raw)
	var buf []rune
	flush := func() {
		if len(buf) == 0 {
			return
		}
		orig := string(buf)
		buf = buf[:0]
		lower := strings.ToLower(orig)
		if stops[lower] {
			return
		}
		base := plainWeight
		if body {
			r, _ := utf8.DecodeRuneInString(orig)
			if unicode.IsUpper(r) {
				base = properNounWeight
			}
		}
		mult := 1.0
		if weights != nil {
			mult = learnedMultiplier(weights[lower])
		}
		vec[lower] += float64(base) * mult
	}
	for _, r := range plain {
		if unicode.IsLetter(r) {
			buf = append(buf, r)
			continue
		}
		flush()
	}
	flush()
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func learnedMultiplier(t TokenTally) float64 {
	w := 1 + 0.25*float64(t.Up-t.Down)
	if w < 0.1 {
		return 0.1
	}
	if w > 4 {
		return 4
	}
	return w
}
