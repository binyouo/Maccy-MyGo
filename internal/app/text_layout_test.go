package app

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSizedPreviewTextByteBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		truncated         bool
	}{
		{"empty", "", "", false},
		{"exactly-10000", strings.Repeat("a", 10_000), strings.Repeat("a", 10_000), false},
		{"over-10000", strings.Repeat("a", 10_001), strings.Repeat("a", 10_000), true},
		{"unicode-exact", strings.Repeat("a", 9_997) + "界", strings.Repeat("a", 9_997) + "界", false},
		{"unicode-overflow", strings.Repeat("a", 9_998) + "界", strings.Repeat("a", 9_998), true},
		{"emoji-overflow", strings.Repeat("a", 9_999) + "😀", strings.Repeat("a", 9_999), true},
		{"unicode-byte-count", strings.Repeat("界", 3_334), strings.Repeat("界", 3_333), true},
		{"stop-entire-preview", "short\n" + strings.Repeat("a", 10_001) + "\nlast", "short\n" + strings.Repeat("a", 10_000), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, truncated := sizedPreviewText(tc.input)
			if got != tc.want || truncated != tc.truncated {
				t.Fatalf("preview bytes=%d truncated=%v; want bytes=%d truncated=%v", len(got), truncated, len(tc.want), tc.truncated)
			}
			if !utf8.ValidString(got) {
				t.Fatal("preview split a UTF-8 scalar")
			}
		})
	}
}

func TestSizedPreviewTextAllNewlinesResetParagraphLimit(t *testing.T) {
	for _, newline := range []string{"\n", "\v", "\f", "\r", "\r\n", "\u0085", "\u2028", "\u2029"} {
		t.Run(strings.ReplaceAll(newline, "\n", "LF"), func(t *testing.T) {
			// Exactly-full paragraphs may end with a newline, even when the
			// complete string is much larger than the per-paragraph limit.
			input := strings.Repeat("a", 10_000) + newline + strings.Repeat("界", 3_000) + newline + "tail"
			if got, truncated := sizedPreviewText(input); got != input || truncated {
				t.Fatalf("newline %q did not reset paragraph byte count", newline)
			}
			input = "head" + newline + strings.Repeat("a", 10_001) + newline + "tail"
			want := "head" + newline + strings.Repeat("a", 10_000)
			if got, truncated := sizedPreviewText(input); got != want || !truncated {
				t.Fatalf("newline %q did not retain the prefix through the first oversized paragraph", newline)
			}
		})
	}
}

func TestSizedPreviewTextDoesNotLimitCombinedShortParagraphs(t *testing.T) {
	input := strings.Repeat(strings.Repeat("x", 100)+"\n", 200)
	if got, truncated := sizedPreviewText(input); got != input || truncated {
		t.Fatal("combined text size was incorrectly used as the paragraph limit")
	}
}

func TestSortPinChoicesLettersBeforeSymbols(t *testing.T) {
	choices := []string{"[", "z", "", "b", "'", "a", "]", "-", "a"}
	sortPinChoices(choices)
	want := []string{"a", "a", "b", "z", "", "'", "-", "[", "]"}
	if !reflect.DeepEqual(choices, want) {
		t.Fatalf("pin choices=%q, want %q", choices, want)
	}
	// Empty and singleton lists require no special caller handling.
	sortPinChoices(nil)
	singleton := []string{""}
	sortPinChoices(singleton)
	if singleton[0] != "" {
		t.Fatal("empty shortcut changed")
	}
}
