package search

import (
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// Foundation searches and Swift Characters use canonical equivalence.
// Keep each transformed cluster tied to the original rune offsets so a
// case fold such as ß -> ss never shifts the highlight onto other text.
type searchCharacters struct {
	text    []string
	offsets []int
}

func characters(s string, lowercase bool, limit int) searchCharacters {
	it := uniseg.NewGraphemes(s)
	out := searchCharacters{}
	lower := cases.Lower(language.Und)
	end := 0
	for it.Next() {
		if limit > 0 && len(out.text) == limit {
			break
		}
		original := it.Str()
		value := original
		if lowercase {
			value = lower.String(value)
		}
		out.text = append(out.text, norm.NFC.String(value))
		out.offsets = append(out.offsets, end)
		end += utf8.RuneCountInString(original)
	}
	out.offsets = append(out.offsets, end)
	return out
}

func foldedText(s string) (string, map[int]int) {
	chars := characters(s, false, 0)
	var text strings.Builder
	boundaries := map[int]int{0: 0}
	fold := cases.Fold()
	for i, ch := range chars.text {
		text.WriteString(norm.NFC.String(fold.String(ch)))
		boundaries[text.Len()] = chars.offsets[i+1]
	}
	return text.String(), boundaries
}

func exactRange(title, query string) (Range, bool) {
	text, boundaries := foldedText(title)
	pattern, _ := foldedText(query)
	if pattern == "" {
		return Range{}, false
	}
	for offset := 0; offset <= len(text)-len(pattern); {
		found := strings.Index(text[offset:], pattern)
		if found < 0 {
			break
		}
		start := offset + found
		end := start + len(pattern)
		lo, starts := boundaries[start]
		hi, ends := boundaries[end]
		if starts && ends {
			return Range{lo, hi}, true
		}
		offset = start + 1
	}
	return Range{}, false
}
