package search

import (
	"strings"
	"unicode"
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

// asciiChar interns one-byte strings so the ASCII fast path does not
// allocate per character.
var asciiChar [256]string

func init() {
	for i := range asciiChar {
		asciiChar[i] = string([]byte{byte(i)})
	}
}

func asciiLower(b byte) string {
	if b >= 'A' && b <= 'Z' {
		b += 'a' - 'A'
	}
	return asciiChar[b]
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func hasCasedRune(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) || unicode.IsTitle(r) {
			return true
		}
	}
	return false
}

func characters(s string, lowercase bool, limit int) searchCharacters {
	if isASCII(s) {
		// ASCII clusters are single bytes, case folding is plain
		// lowering and NFC is identity: byte offsets are rune offsets.
		n := len(s)
		if limit > 0 && n > limit {
			n = limit
		}
		out := searchCharacters{text: make([]string, n), offsets: make([]int, n+1)}
		for i := 0; i < n; i++ {
			c := asciiChar[s[i]]
			if lowercase {
				c = asciiLower(s[i])
			}
			out.text[i] = c
			out.offsets[i] = i
		}
		out.offsets[n] = n
		return out
	}
	it := uniseg.NewGraphemes(s)
	out := searchCharacters{}
	lower := cases.Lower(language.Und)
	// Per-cluster NFC is only needed when the whole string is not
	// already normalized; clusters of NFC text are themselves NFC.
	needsNorm := !norm.NFC.IsNormalString(s)
	end := 0
	for it.Next() {
		if limit > 0 && len(out.text) == limit {
			break
		}
		original := it.Str()
		value := original
		if lowercase && hasCasedRune(value) {
			value = lower.String(value)
		}
		if needsNorm {
			value = norm.NFC.String(value)
		}
		out.text = append(out.text, value)
		out.offsets = append(out.offsets, end)
		end += utf8.RuneCountInString(original)
	}
	out.offsets = append(out.offsets, end)
	return out
}

// foldedText returns the case-folded, NFC form of s along with boundary
// offsets: bounds[i] is the original rune offset ending at folded byte i,
// or -1 when i lands inside a cluster that expanded or contracted. The
// returned slice is nil for pure-ASCII input, where folded text is simply
// lowercased s and every byte offset is a rune boundary.
func foldedText(s string) (string, []int) {
	if isASCII(s) {
		return strings.ToLower(s), nil
	}
	chars := characters(s, false, 0)
	var text strings.Builder
	text.Grow(len(s) + 4)
	bounds := []int{0}
	fold := cases.Fold()
	for i, ch := range chars.text {
		v := ch
		if len(ch) == 1 {
			v = asciiLower(ch[0])
		} else {
			v = fold.String(ch)
			if v != ch {
				v = norm.NFC.String(v)
			}
		}
		text.WriteString(v)
		for len(bounds) <= text.Len() {
			bounds = append(bounds, -1)
		}
		bounds[text.Len()] = chars.offsets[i+1]
	}
	return text.String(), bounds
}

func exactRange(title, pattern string, asciiPattern bool) (Range, bool) {
	if isASCII(title) {
		// A folded pattern with non-ASCII text cannot occur inside
		// pure-ASCII text; otherwise lowering is equivalent to folding.
		if !asciiPattern {
			return Range{}, false
		}
		i := strings.Index(strings.ToLower(title), pattern)
		if i < 0 {
			return Range{}, false
		}
		return Range{i, i + len(pattern)}, true
	}
	text, bounds := foldedText(title)
	for offset := 0; offset <= len(text)-len(pattern); {
		found := strings.Index(text[offset:], pattern)
		if found < 0 {
			break
		}
		start := offset + found
		end := start + len(pattern)
		lo, hi := bounds[start], bounds[end]
		if lo >= 0 && hi >= 0 {
			return Range{lo, hi}, true
		}
		offset = start + 1
	}
	return Range{}, false
}
