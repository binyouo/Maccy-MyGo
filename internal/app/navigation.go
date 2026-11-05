package app

import "github.com/rivo/uniseg"

// Swift's String.popLast removes one Character, including a composed
// accent or an emoji sequence, rather than only its last code point.
func dropLastGrapheme(s string) string {
	last := 0
	for it := uniseg.NewGraphemes(s); it.Next(); {
		last, _ = it.Positions()
	}
	return s[:last]
}

func firstCharacters(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for it := uniseg.NewGraphemes(s); it.Next(); {
		if count == n {
			start, _ := it.Positions()
			return s[:start]
		}
		count++
	}
	return s
}
