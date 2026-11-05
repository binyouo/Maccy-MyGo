package search

import (
	"testing"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

func items() []*model.Item {
	return []*model.Item{
		{Title: "Hello World"},
		{Title: "goodbye world"},
		{Title: "HELLO there"},
		{Title: "unrelated"},
	}
}

func TestExactSearch(t *testing.T) {
	res := Run(items(), "hello", config.SearchExact)
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %d", len(res))
	}
}

func TestRegexpSearch(t *testing.T) {
	res := Run(items(), "^Hello", config.SearchRegexp)
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
}

func TestFuzzySearch(t *testing.T) {
	res := Run(items(), "hlwd", config.SearchFuzzy)
	if len(res) == 0 {
		t.Fatal("expected fuzzy results")
	}
}

func TestMixedFallsBackToFuzzy(t *testing.T) {
	res := Run(items(), "hlo", config.SearchMixed)
	if len(res) == 0 {
		t.Fatal("expected mixed fallback to fuzzy")
	}
}
