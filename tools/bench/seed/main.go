// Command seed writes a synthetic clipboard history in this app's on-disk
// format (history.json + content-addressed blobs) plus a config.json, so a
// benchmark instance can launch with a populated store.
//
// Usage:
//
//	go run ./tools/bench/seed -dir "$HOME/Library/Application Support/MaccyBench" \
//	  -n 200 -corpus corpus.json -imgfrac 0.3
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/binyouo/Maccy-MyGo/internal/model"
)

func main() {
	dir := flag.String("dir", "", "store directory (required)")
	corpus := flag.String("corpus", "", "corpus JSON for titles (optional)")
	n := flag.Int("n", 200, "number of items")
	imgfrac := flag.Float64("imgfrac", 0.3, "fraction of items that also carry a PNG blob")
	flag.Parse()
	if *dir == "" {
		panic("-dir is required")
	}

	var titles []string
	if *corpus != "" {
		data, err := os.ReadFile(*corpus)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &titles); err != nil {
			panic(err)
		}
	}

	store, err := model.Open(*dir)
	if err != nil {
		panic(err)
	}
	r := rand.New(rand.NewSource(7))
	now := time.Now()
	for i := 0; i < *n; i++ {
		title := fmt.Sprintf("seeded bench item %05d lorem ipsum dolor sit amet", i)
		if len(titles) > 0 {
			title = titles[i%len(titles)]
		}
		var contents []model.Content
		if h, size, err := store.PutBlob([]byte(title)); err == nil {
			contents = append(contents, model.Content{Type: "public.utf8-plain-text", Hash: h, Size: size})
		} else {
			panic(err)
		}
		if r.Float64() < *imgfrac {
			pngData := noisePNG(r)
			if h, size, err := store.PutBlob(pngData); err == nil {
				contents = append(contents, model.Content{Type: "public.png", Hash: h, Size: size})
			} else {
				panic(err)
			}
		}
		store.Items = append(store.Items, &model.Item{
			ID:             fmt.Sprintf("seed-%05d", i),
			Application:    "com.bench.seeder",
			FirstCopiedAt:  now.Add(-time.Duration(*n-i) * time.Minute),
			LastCopiedAt:   now.Add(-time.Duration(*n-i) * time.Minute),
			NumberOfCopies: 1 + r.Intn(4),
			Title:          title,
			Contents:       contents,
		})
	}
	if err := store.Save(); err != nil {
		panic(err)
	}

	cfg := map[string]any{"historySize": *n}
	if *n < 200 {
		cfg["historySize"] = 200
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(*dir, "config.json"), data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("seeded %d items into %s\n", *n, *dir)
}

// noisePNG renders a deterministic noisy image, sized like a real screenshot
// fragment so blob weight is realistic.
func noisePNG(r *rand.Rand) []byte {
	const w, h = 480, 320
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(r.Intn(256))
			img.SetRGBA(x, y, color.RGBA{v, uint8(r.Intn(256)), uint8(r.Intn(256)), 255})
		}
	}
	f, err := os.CreateTemp("", "seed-*.png")
	if err != nil {
		panic(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		panic(err)
	}
	return data
}
