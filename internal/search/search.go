// Package search matches a query against history titles, like Maccy:
// exact substring, regular expression, fuzzy, or mixed.
package search

import (
	"sort"
	"time"

	"github.com/dlclark/regexp2"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

// Range is an original-text rune range [Start, End) to highlight.
type Range struct{ Start, End int }

// Result is a matched item with its highlight ranges and score.
type Result struct {
	Item   *model.Item
	Score  float64
	Ranges []Range
}

// Run preserves input order except for fuzzy matches, which Maccy sorts
// by Fuse score. Mixed search falls back only when an entire stage is empty.
func Run(items []*model.Item, query string, mode config.SearchMode) []Result {
	if query == "" {
		out := make([]Result, len(items))
		for i, it := range items {
			out[i] = Result{Item: it}
		}
		return out
	}
	switch mode {
	case config.SearchRegexp:
		return simple(items, query, true)
	case config.SearchFuzzy:
		return fuzzy(items, query)
	case config.SearchMixed:
		if out := simple(items, query, false); len(out) > 0 {
			return out
		}
		if out := simple(items, query, true); len(out) > 0 {
			return out
		}
		return fuzzy(items, query)
	default:
		return simple(items, query, false)
	}
}

func simple(items []*model.Item, query string, useRegexp bool) []Result {
	var out []Result
	if useRegexp {
		// Foundation's regex accepts lookaround and backreferences, unlike
		// Go's RE2. Bound backtracking work; errors/timeouts are not matches.
		rx, err := regexp2.Compile(query, 0)
		if err != nil {
			return nil
		}
		deadline := time.Now().Add(100 * time.Millisecond)
		for _, it := range items {
			rx.MatchTimeout = time.Until(deadline)
			if rx.MatchTimeout <= 0 {
				break
			}
			match, err := rx.FindStringMatch(it.Title)
			if err != nil {
				// A pathological pattern must not consume another timeout
				// for every item in a large history.
				break
			}
			if match == nil {
				continue
			}
			out = append(out, Result{Item: it, Ranges: []Range{{match.Index, match.Index + match.Length}}})
		}
		return out
	}
	for _, it := range items {
		if r, ok := exactRange(it.Title, query); ok {
			out = append(out, Result{Item: it, Ranges: []Range{r}})
		}
	}
	return out
}

func fuzzy(items []*model.Item, query string) []Result {
	pattern := makeFuzzyPattern(query)
	var out []Result
	for _, it := range items {
		ranges, score, ok := pattern.match(it.Title)
		if ok {
			out = append(out, Result{Item: it, Score: score, Ranges: ranges})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score < out[j].Score })
	return out
}
