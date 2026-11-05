#!/usr/bin/env python3
"""report.py — merge /tmp/maccy-bench/out/* into a markdown report.

Usage: python3 tools/bench/report.py [outdir] > RESULTS.md
"""
import json
import re
import statistics
import sys
from pathlib import Path

OUT = Path(sys.argv[1] if len(sys.argv) > 1 else "/tmp/maccy-bench/out")


def load_search(prefix):
    by_n = {}
    for f in sorted(OUT.glob(f"{prefix}-search-*.json")):
        n = int(f.stem.rsplit("-", 1)[1])
        by_n[n] = {(r["mode"], r["query_name"]): r for r in json.loads(f.read_text())}
    return by_n


def fmt_ms(ns):
    return f"{ns / 1e6:.2f}"


def med(vals):
    vals = [v for v in vals if v is not None and v >= 0]
    return statistics.median(vals) if vals else None


def parse_kv_file(path):
    d = {}
    cur = None
    for tok in path.read_text().split():
        if "=" in tok:
            cur, v = tok.split("=", 1)
            d[cur] = [v]
        elif cur is not None:
            d[cur].append(tok)
    return d


def ints(s):
    return [float(x) for x in s.split()] if s else []


go = load_search("go")
sw = load_search("swift")

lines = []
w = lines.append
w("# Maccy 原版 (Swift) vs Go 重写版 性能压测报告")
w("")

# ---------- search ----------
w("## 1. 搜索算法微基准（同语料同查询）")
w("")
w("语料：10,000 条合成标题（25% 英文句 / 15% 代码 / 10% URL / 25% 中文 / 10% 中英混排 / 10% 短词 / 5% 3-7KB 长文本）。")
w("数字为该查询对全表扫描一次的耗时中位数（ms）。⚠️ 标记表示两侧命中数不同——Go 版对正则搜索设有 100ms 总预算，大表 + 昂贵模式会提前截断（设计取舍，见 AUDIT.md）。")
w("")
for n in sorted(set(go) | set(sw)):
    g, s = go.get(n, {}), sw.get(n, {})
    w(f"### n = {n}")
    w("")
    w("| 模式 | 查询 | Go ms | Swift ms | Go/Swift | Go 命中 | Swift 命中 |")
    w("| --- | --- | --- | --- | --- | --- | --- |")
    for mode in ("exact", "regexp", "fuzzy", "mixed"):
        queries = [q for (m, q) in g if m == mode] or [q for (m, q) in s if m == mode]
        for q in queries:
            a, b = g.get((mode, q)), s.get((mode, q))
            if not a or not b:
                continue
            ratio = a["median_ns"] / max(b["median_ns"], 1)
            flag = " ⚠️" if a["matches"] != b["matches"] else ""
            w(f"| {mode} | {q} | {fmt_ms(a['median_ns'])} | {fmt_ms(b['median_ns'])} "
              f"| {ratio:.2f}× | {a['matches']} | {b['matches']}{flag} |")
    w("")

# ---------- app level ----------
def section_launch(tag, label):
    p = OUT / f"{tag}-launch.txt"
    if not p.exists():
        return
    d = parse_kv_file(p)
    w(f"**{label}**")
    w("")
    w("| 指标 | 样本 | 中位数 |")
    w("| --- | --- | --- |")
    for key, name in (("t_dbopen_ms", "存储就绪"), ("t_sock_ms", "socket 就绪"),
                      ("t_quiet_ms", "CPU 安静"), ("rss_kb", "RSS"),
                      ("footprint_kb", "phys_footprint"), ("idle_cpu_pct", "空闲 CPU%")):
        if key in d:
            vs = ints(" ".join(d[key]))
            m = med(vs)
            unit = " KB" if "kb" in key else (" ms" if "ms" in key else " %")
            w(f"| {name} | {' '.join(d[key])} | {m:.0f}{unit} |")
    w("")


w("## 2. 应用级指标")
w("")
section_launch("upstream", "原版 Maccy（真实 200 条 / 1.8GB 历史）")
section_launch("go", "Go 版（合成 200 条历史）")

p = OUT / "capture.txt"
if p.exists():
    d = parse_kv_file(p)
    w("**剪贴板捕获延迟（ms，写入 → 历史可见）**")
    w("")
    for k in ("upstream", "go"):
        if k in d:
            vs = ints(" ".join(d[k]))
            m = med(vs)
            w(f"- {k}: median={m:.0f}  min={min(vs) if vs else '-'}  max={max(vs) if vs else '-'}  n={len(vs)}")
    w("")
    w("注：两侧默认轮询间隔均为 0.5s，故延迟呈 0-500ms 均匀分布，中位数 ≈ 250ms 为理论值。")
    w("")

p = OUT / "ui-search.txt"
if p.exists():
    w("**Go 版 UI 搜索延迟**")
    w("")
    w("```")
    w(p.read_text().strip())
    w("```")
    w("含 automation IPC 与渲染轮询开销，仅作参考量级。")
    w("")

p = OUT / "sizes.txt"
if p.exists():
    w("**体积**")
    w("")
    w("```")
    w(p.read_text().strip())
    w("```")
    w("")

print("\n".join(lines))
