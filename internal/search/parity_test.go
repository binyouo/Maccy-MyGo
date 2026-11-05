package search

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

// These scores and ranges are the vectors in the checked-in Swift
// MaccyTests/SearchTests.swift, produced by its pinned Fuse 1.4.0.
func TestFuzzyMatchesMaccyReferenceVectors(t *testing.T) {
	titles := []*model.Item{{Title: "foo bar baz"}, {Title: "foo bar zaz"}, {Title: "xxx yyy zzz"}}
	for _, tc := range []struct {
		query   string
		indexes []int
		scores  []float64
		ranges  [][]Range
	}{
		{"z", []int{1, 2, 0}, []float64{.08, .08, .1}, [][]Range{{{8, 9}, {10, 11}}, {{8, 11}}, {{10, 11}}}},
		{"foo", []int{0, 1}, []float64{0, 0}, [][]Range{{{0, 3}}, {{0, 3}}}},
		{"za", []int{1, 0, 2}, []float64{.08, .54, .58}, [][]Range{{{5, 6}, {8, 10}}, {{5, 6}, {9, 11}}, {{8, 11}}}},
		{"yyy", []int{2}, []float64{.04}, [][]Range{{{4, 7}}}},
		{"fbb", []int{0, 1}, []float64{2.0 / 3, 2.0 / 3}, [][]Range{{{0, 1}, {4, 5}, {8, 9}}, {{0, 1}, {4, 5}}}},
		{"m", nil, nil, nil},
	} {
		t.Run(tc.query, func(t *testing.T) {
			got := Run(titles, tc.query, config.SearchFuzzy)
			if len(got) != len(tc.indexes) {
				t.Fatalf("got %d results, want %d: %+v", len(got), len(tc.indexes), got)
			}
			for i, r := range got {
				if r.Item != titles[tc.indexes[i]] || math.Abs(r.Score-tc.scores[i]) > 1e-12 || !reflect.DeepEqual(r.Ranges, tc.ranges[i]) {
					t.Errorf("result %d: title=%q score=%v ranges=%v; want title=%q score=%v ranges=%v", i, r.Item.Title, r.Score, r.Ranges, titles[tc.indexes[i]].Title, tc.scores[i], tc.ranges[i])
				}
			}
		})
	}
}

func TestExactUnicodeCaseFoldingAndOriginalRanges(t *testing.T) {
	for _, tc := range []struct {
		title, query string
		want         []Range
	}{
		{"Straße", "STRASSE", []Range{{0, 6}}},
		{"Straße", "ss", []Range{{4, 5}}},
		{"e\u0301cole", "É", []Range{{0, 2}}},
		{"Kello", "ello", []Range{{1, 5}}},
		{"Khello", "llo", []Range{{3, 6}}},
		{"İstanbul", "an", []Range{{3, 5}}},
		{"ſoo", "SOO", []Range{{0, 3}}},
		{"ΟΣ", "ος", []Range{{0, 2}}},
		{"👩‍💻abc", "abc", []Range{{3, 6}}},
		{"café", "cafe", nil},
		{"İstanbul", "i", nil},
		{"ß", "s", nil},
		{"👩‍💻", "💻", nil},
	} {
		t.Run(tc.title+"/"+tc.query, func(t *testing.T) {
			got := Run([]*model.Item{{Title: tc.title}}, tc.query, config.SearchExact)
			if tc.want == nil {
				if len(got) != 0 {
					t.Fatalf("unexpected match: %+v", got)
				}
				return
			}
			if len(got) != 1 || !reflect.DeepEqual(got[0].Ranges, tc.want) {
				t.Fatalf("got %+v, want ranges %v", got, tc.want)
			}
		})
	}
}

func TestFuzzyUnicodeGraphemeOffsets(t *testing.T) {
	got := Run([]*model.Item{{Title: "👩‍💻abc"}}, "abc", config.SearchFuzzy)
	if len(got) != 1 || got[0].Score != .01 || !reflect.DeepEqual(got[0].Ranges, []Range{{3, 6}}) {
		t.Fatalf("unexpected emoji offsets/score: %+v", got)
	}
}

func TestRepeatedEmojiAndFlagGraphemeOffsets(t *testing.T) {
	for _, tc := range []struct {
		title string
		want  Range
	}{
		{strings.Repeat("👩‍💻", 3) + "abc", Range{9, 12}},
		{"🇨🇳🇯🇵🇺🇸abc", Range{6, 9}},
	} {
		t.Run(tc.title, func(t *testing.T) {
			for _, mode := range []config.SearchMode{config.SearchExact, config.SearchFuzzy} {
				got := Run([]*model.Item{{Title: tc.title}}, "abc", mode)
				if len(got) != 1 || !reflect.DeepEqual(got[0].Ranges, []Range{tc.want}) {
					t.Fatalf("%s: got %+v, want rune offsets %v", mode, got, tc.want)
				}
				if mode == config.SearchFuzzy && got[0].Score != .03 {
					t.Fatalf("repeated emoji/flags must count as three Swift Characters, got score %v", got[0].Score)
				}
			}
		})
	}
	for _, mode := range []config.SearchMode{config.SearchExact, config.SearchFuzzy} {
		got := Run([]*model.Item{{Title: strings.Repeat("👩‍💻", 3)}}, "💻", mode)
		if len(got) != 0 {
			t.Fatalf("%s matched a laptop scalar inside a repeated joined emoji", mode)
		}
	}
}

func TestRegexpLookaroundBackreferenceAndRuneOffsets(t *testing.T) {
	for _, tc := range []struct {
		title, query string
		want         Range
	}{
		{"foo bar", "foo(?= bar)", Range{0, 3}},
		{"foo bar", "(?<=foo )bar", Range{4, 7}},
		{"echo echo", `\b(\w+) \1\b`, Range{0, 9}},
		{"你好 hello hello", `(hello) \1`, Range{3, 14}},
		{"你好", "(?=你)", Range{0, 0}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			got := Run([]*model.Item{{Title: tc.title}}, tc.query, config.SearchRegexp)
			if len(got) != 1 || !reflect.DeepEqual(got[0].Ranges, []Range{tc.want}) {
				t.Fatalf("got %+v, want range %+v", got, tc.want)
			}
		})
	}
	if got := Run(items(), "[", config.SearchRegexp); len(got) != 0 {
		t.Fatal("invalid regex matched")
	}
}

func TestMixedSearchUsesWholeStageFallback(t *testing.T) {
	titles := []*model.Item{{Title: "foo bar"}, {Title: "foo(?= bar)"}, {Title: "food"}}
	got := Run(titles, "foo(?= bar)", config.SearchMixed)
	if len(got) != 1 || got[0].Item != titles[1] {
		t.Fatal("literal matches did not take precedence over regexp/fuzzy")
	}
	got = Run(titles[:1], "foo(?= bar)", config.SearchMixed)
	if len(got) != 1 || !reflect.DeepEqual(got[0].Ranges, []Range{{0, 3}}) {
		t.Fatal("mixed regexp stage did not accept lookahead")
	}
	got = Run([]*model.Item{{Title: "hello"}}, "helo", config.SearchMixed)
	if len(got) != 1 {
		t.Fatal("mixed search did not fall back to typo-tolerant fuzzy search")
	}
}

func TestRegexpSearchTimeoutIsNoMatch(t *testing.T) {
	items := make([]*model.Item, 20)
	for i := range items {
		items[i] = &model.Item{Title: strings.Repeat("a", 40) + "!"}
	}
	start := time.Now()
	got := Run(items, "(a+)+$", config.SearchRegexp)
	if len(got) != 0 {
		t.Fatal("timed-out expression matched")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("backtracking was not bounded: %v", elapsed)
	}
}
