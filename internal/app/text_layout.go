package app

import (
	"sort"
	"unicode"
	"unicode/utf8"
)

const previewParagraphBytes = 10_000

// sizedPreviewText follows SizedString's paragraph-byte bound. The first
// oversized paragraph ends the entire preview, rather than removing text
// independently from each paragraph. A boundary never splits a UTF-8 scalar.
func sizedPreviewText(text string) (string, bool) {
	paragraphBytes := 0
	for cursor := 0; cursor < len(text); {
		if n := previewLineBreakLength(text, cursor); n != 0 {
			cursor += n
			paragraphBytes = 0
			continue
		}
		if paragraphBytes == previewParagraphBytes {
			end := cursor
			for end > 0 && text[end]&0xc0 == 0x80 {
				end--
			}
			return text[:end], true
		}
		paragraphBytes++
		cursor++
	}
	return text, false
}

// These are the same newline sequences recognized by Maccy's
// String.UTF8View.indexAfterLineBreak, including CRLF as one break.
func previewLineBreakLength(text string, at int) int {
	switch text[at] {
	case '\n', '\v', '\f':
		return 1
	case '\r':
		if at+1 < len(text) && text[at+1] == '\n' {
			return 2
		}
		return 1
	case 0xc2:
		if at+1 < len(text) && text[at+1] == 0x85 {
			return 2
		}
	case 0xe2:
		if at+2 < len(text) && text[at+1] == 0x80 && (text[at+2] == 0xa8 || text[at+2] == 0xa9) {
			return 3
		}
	}
	return 0
}

// sortPinChoices sorts in place like HistoryItem.sortedPins: letters
// precede symbols, then each group is ordered lexicographically.
func sortPinChoices(choices []string) {
	isLetter := func(value string) bool {
		first, _ := utf8.DecodeRuneInString(value)
		return unicode.IsLetter(first)
	}
	sort.SliceStable(choices, func(i, j int) bool {
		iLetter, jLetter := isLetter(choices[i]), isLetter(choices[j])
		if iLetter != jLetter {
			return iLetter
		}
		return choices[i] < choices[j]
	})
}
