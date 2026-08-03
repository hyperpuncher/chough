package tokens

import (
	"strings"
	"unicode"
)

type Unit struct {
	Start int
	End   int
}

// Units groups subword tokens into words and keeps punctuation with the
// preceding word. Languages without space-delimited tokens use one unit per
// non-punctuation token.
func Units(values []string) []Unit {
	usesSpaces := false
	for _, value := range values {
		if StartsWord(value) {
			usesSpaces = true
			break
		}
	}

	units := make([]Unit, 0, len(values))
	for index, value := range values {
		startsUnit := index == 0 || (usesSpaces && StartsWord(value)) || (!usesSpaces && !IsPunctuation(value))
		if startsUnit {
			units = append(units, Unit{Start: index, End: index})
		} else {
			units[len(units)-1].End = index
		}
	}
	return units
}

func StartsWord(value string) bool {
	for _, r := range value {
		return unicode.IsSpace(r)
	}
	return false
}

func IsPunctuation(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			return false
		}
	}
	return true
}
