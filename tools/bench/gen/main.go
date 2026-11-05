// Command gen builds the benchmark corpus and query set.
//
// The corpus is synthetic but follows the mix seen in real Maccy histories:
// English prose, code fragments, URLs/paths, CJK text, short tokens and a
// small number of very long items. A deterministic seed keeps runs
// comparable. A handful of "needle" titles are placed at fixed positions so
// hit queries have known matches.
//
// Usage:
//
//	go run ./tools/bench/gen -out /tmp/maccy-bench/data
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

var enWords = []string{
	"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog", "clipboard",
	"history", "manager", "window", "keyboard", "shortcut", "search", "result",
	"memory", "build", "release", "debug", "compile", "runtime", "goroutine",
	"channel", "interface", "struct", "package", "import", "function", "return",
	"error", "value", "pointer", "slice", "map", "string", "buffer", "cache",
	"server", "client", "request", "response", "header", "token", "session",
	"commit", "branch", "merge", "review", "deploy", "container", "docker",
	"database", "query", "index", "table", "column", "transaction", "schema",
	"algorithm", "benchmark", "latency", "throughput", "profile", "trace",
	"mutex", "deadlock", "race", "condition", "atomic", "sequential", "random",
	"persistent", "ephemeral", "snapshot", "backup", "restore", "migrate",
	"upgrade", "downgrade", "version", "semver", "dependency", "module",
}

var codeSnippets = []string{
	"func TestBench(t *testing.T) { t.Parallel() }",
	"if err != nil { return fmt.Errorf(\"load: %w\", err) }",
	"SELECT id, title FROM history WHERE pinned = 1 ORDER BY last_copied_at DESC",
	"git rebase -i HEAD~3 && git push --force-with-lease",
	"docker run --rm -p 8080:80 nginx:alpine",
	"for i := range items { wg.Go(func() { process(items[i]) }) }",
	"CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '-s -w'",
	"curl -sSf https://example.com/api/v2/items?limit=50&offset=0",
	"NSPasteboard.general().setString(text, forType: .string)",
	"let fuse = Fuse(threshold: 0.7) // threshold found by trial-and-error",
	"swiftc -O -whole-module-optimization -o bench main.swift",
	"defaults write org.p0deje.Maccy clipboardCheckInterval -float 0.5",
	"INSERT INTO ZHISTORYITEM (ZTITLE) VALUES (?)",
	"regexp2.Compile(`\\d{4}-\\d{2}-\\d{2}`, regexp2.None)",
	"ssh -o StrictHostKeyChecking=no deploy@prod-01.internal",
}

var urls = []string{
	"https://github.com/p0deje/Maccy/releases/tag/2.7.1",
	"https://pkg.go.dev/github.com/ebitengine/purego",
	"https://developer.apple.com/documentation/appkit/nspasteboard",
	"https://stackoverflow.com/questions/12345/fuzzy-search-bitap-algorithm",
	"file:///Users/think/Documents/specs/clipboard-formats-v3.pdf",
	"https://fusejs.io/api/options.html#threshold",
	"https://localhost:3000/settings/storage?tab=advanced",
	"ssh://git@gitlab.internal:2222/platform/clipboard-service.git",
	"https://news.ycombinator.com/item?id=38291011",
	"/Volumes/T7/MyGoMaccy/internal/search/fuzzy.go",
	"~/Library/Application Support/Maccy/Storage.sqlite",
	"/usr/local/Cellar/go/1.27.3/libexec/src/strings/strings.go",
}

var cjkSentences = []string{
	"剪贴板历史记录管理器测试基准数据集",
	"兄弟们其实真的可以休息了义务教育几年时间",
	"今天的会议纪要：性能压测对比报告需要在周五前完成",
	"这是一个包含中文和 English 混合的剪贴板条目 example",
	"麻老爷舍哈哈小猴猫现在一个人了",
	"验证结果已读取文件样例数据编码格式检查通过",
	"使用全局快捷键呼出窗口然后输入关键字过滤历史",
	"正则表达式匹配前后查找和反向引用的边界情况",
	"工具的执行结果未知请先检查实际结果再决定是否重试",
	"义务教育几年时间里连二十六个字母都没背全",
	"名称数量备注咖啡茶中文空格符号测试",
	"把剪贴板内容同步到云端的方案还在评审中",
}

func sentence(r *rand.Rand, words []string, min, max int) string {
	n := min + r.Intn(max-min+1)
	out := make([]byte, 0, n*8)
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ' ')
		}
		w := words[r.Intn(len(words))]
		if r.Intn(8) == 0 {
			w = string([]byte{w[0] - 32}) + w[1:]
		}
		out = append(out, w...)
	}
	return string(out)
}

func longText(r *rand.Rand) string {
	out := make([]byte, 0, 6000)
	for len(out) < 3000+r.Intn(4000) {
		out = append(out, sentence(r, enWords, 8, 20)...)
		out = append(out, '.', ' ')
	}
	return string(out)
}

// needles are placed at fixed indices; queries reference their contents.
var needles = map[int]string{
	3:    "needle-alpha-42 unique marker for benchmark queries",
	97:   "needle beta release 2026-11-05 build artifacts",
	211:  "café au lait résumé naïve façade unicode needle",
	499:  "剪贴板历史 needle gamma 查找测试条目",
	777:  "func TestBench needle delta benchmark fixture",
	1234: "contact dev-team@example-corp.com for needle epsilon access",
	5555: "needle eta very deep item in large histories",
	7777: "needle theta mixed 混合 content English and 中文",
	9999: "needle iota final sentinel entry at corpus end",
}

func main() {
	out := flag.String("out", "/tmp/maccy-bench/data", "output directory")
	n := flag.Int("n", 10000, "corpus size")
	flag.Parse()

	r := rand.New(rand.NewSource(42))
	// A long needle built after rng init to keep it deterministic.
	needles[2048] = "needle zeta long tail entry " + longText(r)

	titles := make([]string, *n)
	for i := range titles {
		if t, ok := needles[i]; ok {
			titles[i] = t
			continue
		}
		switch x := r.Intn(100); {
		case x < 25:
			titles[i] = sentence(r, enWords, 4, 14)
		case x < 40:
			titles[i] = codeSnippets[r.Intn(len(codeSnippets))] + " " + sentence(r, enWords, 0, 4)
		case x < 50:
			titles[i] = urls[r.Intn(len(urls))]
		case x < 75:
			titles[i] = cjkSentences[r.Intn(len(cjkSentences))]
			if r.Intn(3) == 0 {
				titles[i] += cjkSentences[r.Intn(len(cjkSentences))]
			}
		case x < 85:
			titles[i] = cjkSentences[r.Intn(len(cjkSentences))] + " " + sentence(r, enWords, 2, 6)
		case x < 95:
			titles[i] = enWords[r.Intn(len(enWords))]
		default:
			titles[i] = longText(r)
		}
	}

	type query struct {
		Name  string `json:"name"`
		Query string `json:"query"`
	}
	queries := []query{
		{"hit-english", "needle-alpha-42"},
		{"hit-partial", "needle"},
		{"hit-cjk", "剪贴板历史"},
		{"hit-code", "func TestBench"},
		{"hit-unicode", "café"},
		{"hit-casefold", "NEEDLE-ALPHA"},
		{"hit-long-item", "long tail entry"},
		{"miss", "zzqxxv-nothing-matches-this"},
		{"fuzzy-hit", "ndlalp"},
		{"fuzzy-word", "clipbrd"},
		{"fuzzy-cjk", "剪历史"},
		{"fuzzy-miss", "qxjvkwpx"},
		{"regex-date", `\d{4}-\d{2}-\d{2}`},
		{"regex-email", `[\w.+-]+@[\w-]+\.[\w.]+`},
		{"regex-prefix", "^https://"},
		{"regex-miss", `z{3}q{2}[^a-z]*$`},
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}
	writeJSON(filepath.Join(*out, "corpus.json"), titles)
	writeJSON(filepath.Join(*out, "queries.json"), queries)
	fmt.Printf("wrote %d titles and %d queries to %s\n", len(titles), len(queries), *out)
}

func writeJSON(path string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(err)
	}
}
