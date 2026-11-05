// Command searchgo benchmarks this project's search implementation against a
// shared corpus and query set, so it can be compared with the Swift harness
// (tools/bench/swiftsearch/main.swift) that mirrors upstream Maccy.
//
// Usage:
//
//	go run ./tools/bench/searchgo -corpus corpus.json -queries queries.json \
//	  -n 200 -reps 20 -modes exact,regexp,fuzzy,mixed
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/binyouo/Maccy-MyGo/internal/search"
)

type query struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

type result struct {
	Mode     string `json:"mode"`
	Query    string `json:"query_name"`
	Matches  int    `json:"matches"`
	MinNs    int64  `json:"min_ns"`
	MedianNs int64  `json:"median_ns"`
	P95Ns    int64  `json:"p95_ns"`
}

func main() {
	corpus := flag.String("corpus", "/tmp/maccy-bench/data/corpus.json", "corpus JSON (array of titles)")
	queriesF := flag.String("queries", "/tmp/maccy-bench/data/queries.json", "queries JSON")
	n := flag.Int("n", 0, "use only the first n titles (0 = all)")
	reps := flag.Int("reps", 20, "timed repetitions")
	warmup := flag.Int("warmup", 3, "warmup repetitions")
	modes := flag.String("modes", "exact,regexp,fuzzy,mixed", "modes to run")
	flag.Parse()

	var titles []string
	readJSON(*corpus, &titles)
	if *n > 0 && *n < len(titles) {
		titles = titles[:*n]
	}
	var queries []query
	readJSON(*queriesF, &queries)

	items := make([]*model.Item, len(titles))
	for i, t := range titles {
		items[i] = &model.Item{Title: t}
	}

	modeMap := map[string]config.SearchMode{
		"exact": config.SearchExact, "regexp": config.SearchRegexp,
		"fuzzy": config.SearchFuzzy, "mixed": config.SearchMixed,
	}
	var out []result
	for _, m := range strings.Split(*modes, ",") {
		mode, ok := modeMap[m]
		if !ok {
			continue
		}
		for _, q := range queries {
			for i := 0; i < *warmup; i++ {
				search.Run(items, q.Query, mode)
			}
			ds := make([]int64, *reps)
			matches := 0
			for i := range ds {
				t0 := time.Now()
				matches = len(search.Run(items, q.Query, mode))
				ds[i] = time.Since(t0).Nanoseconds()
			}
			sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
			out = append(out, result{
				Mode: m, Query: q.Name, Matches: matches,
				MinNs: ds[0], MedianNs: ds[len(ds)/2], P95Ns: ds[len(ds)*19/20],
			})
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
	fmt.Fprintf(os.Stderr, "corpus=%d queries=%d reps=%d\n", len(titles), len(queries), *reps)
}

func readJSON(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		panic(err)
	}
}
