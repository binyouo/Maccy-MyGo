# Maccy 原版 (Swift) vs Go 重写版 性能压测报告

> 2026-11-05 · macOS arm64 · 原版 = `/Applications/Maccy.app` 2.7.1（真实 200 条 / 1.8GB 历史库）·
> 现版 = 本仓库 `CGO_ENABLED=0 -trimpath -ldflags "-s -w"` 构建 · 原始数据见 `/tmp/maccy-bench/out/`，
> 复测方法见 [README.md](README.md)

## 结论摘要

| 维度 | 原版 (Swift) | Go 版 | 结论 |
| --- | --- | --- | --- |
| 启动至空闲 | ~1011 ms | ~506 ms | **Go 快 ~2×**（socket 就绪 95ms vs 原版库打开 282ms） |
| 空闲内存 | RSS 264MB / footprint 223MB | RSS 82MB / footprint 30MB | **Go 省 ~3-7×**（原版背着真实 1.8GB 库，但 SwiftData 常驻开销本身就高） |
| 空闲 CPU | ~0% | ~0.1% | 持平，均可忽略 |
| 剪贴板捕获延迟 | 中位 280ms | 中位 264ms | **持平**（同为 0.5s 轮询，延迟即轮询相位） |
| 搜索·exact | 6.7ms @200 | 11.7ms @200 | 原版快 ~1.7×（NSString 比较高度优化） |
| 搜索·regexp | 1.3-5.2ms @200 | ~0.1ms @200 | **Go 快 10-30×**（regexp2 编译匹配优于 ICU/NSRegularExpression） |
| 搜索·fuzzy | 2.5-5ms @200 | 9.5ms @200 | 原版快 ~2-4×（fuse-swift 在 UTF-16 单元上跑 bitap） |
| 搜索·大表 | n=10000 exact 280ms / fuzzy ~300ms | n=10000 exact 560ms / fuzzy ~500ms | 原版快 ~2×；两者在 200 条真实规模下都远低于感知阈值 |
| UI 搜索端到端 | 无法测（无自动化接口） | ~50ms/查询 | 参考量级，含 IPC |
| 体积 | 二进制 3.8MB / bundle 10MB | 二进制 16.1MB / bundle 15MB | 原版更小（共享系统 Swift/AppKit 框架） |

### 值得注意的点

1. **搜索算法各有胜负**：Go 的 exact/fuzzy 慢 ~1.7-4×（Unicode 大小写折叠与 grapheme 处理的代价），
   但 regexp 快一个数量级。在真实 200 条规模下两边都是 ~10ms 级，无感知差异；
   差异只在超大历史（>2000 条）才显著。
2. **⚠️ 命中数分歧是设计取舍不是 bug**：`regexp`/`mixed` 的 `regex-email` 在 n≥2000 时
   Go 命中更少——Go 对正则搜索设了 100ms 总预算（`internal/search`），昂贵模式扫到一半截断；
   原版 ICU 扫全表。这是响应性 vs 完整性的取舍，AUDIT.md 已记录。
3. **启动与内存优势明显**：Go 版常驻 footprint ~30MB，约为原版 1/7；启动到可用 ~0.5s。
4. **二进制更大**的代价换到的是单文件无依赖分发 + 纯 Go 构建链。

---

以下明细由 `tools/bench/report.py` 生成。

# Maccy 原版 (Swift) vs Go 重写版 性能压测报告

## 1. 搜索算法微基准（同语料同查询）

语料：10,000 条合成标题（25% 英文句 / 15% 代码 / 10% URL / 25% 中文 / 10% 中英混排 / 10% 短词 / 5% 3-7KB 长文本）。
数字为该查询对全表扫描一次的耗时中位数（ms）。⚠️ 标记表示两侧命中数不同——Go 版对正则搜索设有 100ms 总预算，大表 + 昂贵模式会提前截断（设计取舍，见 AUDIT.md）。

### n = 200

| 模式 | 查询 | Go ms | Swift ms | Go/Swift | Go 命中 | Swift 命中 |
| --- | --- | --- | --- | --- | --- | --- |
| exact | hit-english | 11.93 | 6.72 | 1.78× | 1 | 1 |
| exact | hit-partial | 11.56 | 6.82 | 1.70× | 2 | 2 |
| exact | hit-cjk | 12.06 | 12.10 | 1.00× | 6 | 6 |
| exact | hit-code | 12.16 | 6.60 | 1.84× | 1 | 1 |
| exact | hit-unicode | 11.51 | 6.58 | 1.75× | 0 | 0 |
| exact | hit-casefold | 11.70 | 7.02 | 1.67× | 1 | 1 |
| exact | hit-long-item | 11.78 | 6.75 | 1.74× | 0 | 0 |
| exact | miss | 11.97 | 6.67 | 1.80× | 0 | 0 |
| exact | fuzzy-hit | 11.44 | 6.54 | 1.75× | 0 | 0 |
| exact | fuzzy-word | 11.52 | 6.77 | 1.70× | 0 | 0 |
| exact | fuzzy-cjk | 11.56 | 12.12 | 0.95× | 0 | 0 |
| exact | fuzzy-miss | 11.69 | 6.48 | 1.80× | 0 | 0 |
| exact | regex-date | 12.12 | 6.60 | 1.83× | 2 | 2 |
| exact | regex-email | 12.28 | 6.62 | 1.86× | 0 | 0 |
| exact | regex-prefix | 11.75 | 6.46 | 1.82× | 0 | 0 |
| exact | regex-miss | 12.00 | 6.47 | 1.86× | 0 | 0 |
| regexp | hit-english | 0.10 | 1.29 | 0.08× | 1 | 1 |
| regexp | hit-partial | 0.12 | 1.27 | 0.09× | 2 | 2 |
| regexp | hit-cjk | 0.10 | 2.90 | 0.03× | 6 | 6 |
| regexp | hit-code | 0.11 | 1.25 | 0.09× | 1 | 1 |
| regexp | hit-unicode | 0.13 | 2.09 | 0.06× | 0 | 0 |
| regexp | hit-casefold | 0.09 | 1.25 | 0.07× | 0 | 0 |
| regexp | hit-long-item | 0.11 | 1.26 | 0.09× | 0 | 0 |
| regexp | miss | 0.10 | 1.25 | 0.08× | 0 | 0 |
| regexp | fuzzy-hit | 0.12 | 1.28 | 0.10× | 0 | 0 |
| regexp | fuzzy-word | 0.12 | 1.28 | 0.09× | 0 | 0 |
| regexp | fuzzy-cjk | 0.11 | 2.87 | 0.04× | 0 | 0 |
| regexp | fuzzy-miss | 0.12 | 1.21 | 0.10× | 0 | 0 |
| regexp | regex-date | 1.09 | 3.13 | 0.35× | 1 | 1 |
| regexp | regex-email | 18.24 | 32.11 | 0.57× | 4 | 4 |
| regexp | regex-prefix | 0.08 | 0.07 | 1.17× | 15 | 15 |
| regexp | regex-miss | 0.11 | 5.20 | 0.02× | 0 | 0 |
| fuzzy | hit-english | 9.99 | 4.88 | 2.05× | 3 | 3 |
| fuzzy | hit-partial | 9.46 | 2.73 | 3.46× | 83 | 83 |
| fuzzy | hit-cjk | 9.10 | 2.62 | 3.48× | 15 | 15 |
| fuzzy | hit-code | 9.82 | 5.08 | 1.94× | 34 | 34 |
| fuzzy | hit-unicode | 9.12 | 2.18 | 4.18× | 11 | 11 |
| fuzzy | hit-casefold | 9.78 | 4.28 | 2.28× | 17 | 17 |
| fuzzy | hit-long-item | 9.96 | 5.47 | 1.82× | 50 | 50 |
| fuzzy | miss | 10.79 | 8.71 | 1.24× | 0 | 0 |
| fuzzy | fuzzy-hit | 9.41 | 2.72 | 3.46× | 45 | 45 |
| fuzzy | fuzzy-word | 9.48 | 3.00 | 3.17× | 18 | 18 |
| fuzzy | fuzzy-cjk | 9.07 | 2.10 | 4.32× | 16 | 16 |
| fuzzy | fuzzy-miss | 9.52 | 2.75 | 3.46× | 0 | 0 |
| fuzzy | regex-date | 10.05 | 5.17 | 1.94× | 2 | 2 |
| fuzzy | regex-email | 10.52 | 7.17 | 1.47× | 0 | 0 |
| fuzzy | regex-prefix | 9.49 | 3.11 | 3.05× | 20 | 20 |
| fuzzy | regex-miss | 9.98 | 5.13 | 1.94× | 0 | 0 |
| mixed | hit-english | 11.96 | 6.68 | 1.79× | 1 | 1 |
| mixed | hit-partial | 11.56 | 6.63 | 1.74× | 2 | 2 |
| mixed | hit-cjk | 11.64 | 12.08 | 0.96× | 6 | 6 |
| mixed | hit-code | 11.71 | 6.46 | 1.81× | 1 | 1 |
| mixed | hit-unicode | 21.16 | 10.72 | 1.97× | 11 | 11 |
| mixed | hit-casefold | 11.77 | 6.88 | 1.71× | 1 | 1 |
| mixed | hit-long-item | 22.20 | 13.22 | 1.68× | 50 | 50 |
| mixed | miss | 23.26 | 16.34 | 1.42× | 0 | 0 |
| mixed | fuzzy-hit | 21.33 | 10.47 | 2.04× | 45 | 45 |
| mixed | fuzzy-word | 21.44 | 10.89 | 1.97× | 18 | 18 |
| mixed | fuzzy-cjk | 21.45 | 17.34 | 1.24× | 16 | 16 |
| mixed | fuzzy-miss | 21.20 | 10.73 | 1.98× | 0 | 0 |
| mixed | regex-date | 11.96 | 6.50 | 1.84× | 2 | 2 |
| mixed | regex-email | 30.59 | 39.31 | 0.78× | 4 | 4 |
| mixed | regex-prefix | 11.76 | 6.55 | 1.79× | 15 | 15 |
| mixed | regex-miss | 22.92 | 16.60 | 1.38× | 0 | 0 |

### n = 2000

| 模式 | 查询 | Go ms | Swift ms | Go/Swift | Go 命中 | Swift 命中 |
| --- | --- | --- | --- | --- | --- | --- |
| exact | hit-english | 80.13 | 43.96 | 1.82× | 1 | 1 |
| exact | hit-partial | 76.34 | 43.67 | 1.75× | 6 | 6 |
| exact | hit-cjk | 77.77 | 77.90 | 1.00× | 71 | 71 |
| exact | hit-code | 80.14 | 42.85 | 1.87× | 19 | 19 |
| exact | hit-unicode | 76.31 | 43.21 | 1.77× | 1 | 1 |
| exact | hit-casefold | 79.31 | 44.72 | 1.77× | 1 | 1 |
| exact | hit-long-item | 79.68 | 43.19 | 1.85× | 0 | 0 |
| exact | miss | 82.93 | 41.92 | 1.98× | 0 | 0 |
| exact | fuzzy-hit | 77.03 | 43.79 | 1.76× | 0 | 0 |
| exact | fuzzy-word | 77.47 | 43.60 | 1.78× | 0 | 0 |
| exact | fuzzy-cjk | 78.45 | 78.35 | 1.00× | 0 | 0 |
| exact | fuzzy-miss | 76.87 | 43.25 | 1.78× | 0 | 0 |
| exact | regex-date | 80.52 | 43.62 | 1.85× | 17 | 17 |
| exact | regex-email | 83.01 | 43.03 | 1.93× | 0 | 0 |
| exact | regex-prefix | 78.40 | 42.39 | 1.85× | 0 | 0 |
| exact | regex-miss | 81.32 | 41.99 | 1.94× | 0 | 0 |
| regexp | hit-english | 0.78 | 11.99 | 0.07× | 1 | 1 |
| regexp | hit-partial | 0.97 | 8.24 | 0.12× | 6 | 6 |
| regexp | hit-cjk | 0.76 | 19.09 | 0.04× | 71 | 71 |
| regexp | hit-code | 0.82 | 8.75 | 0.09× | 19 | 19 |
| regexp | hit-unicode | 1.00 | 12.80 | 0.08× | 1 | 1 |
| regexp | hit-casefold | 0.74 | 8.40 | 0.09× | 0 | 0 |
| regexp | hit-long-item | 0.81 | 8.13 | 0.10× | 0 | 0 |
| regexp | miss | 0.77 | 8.56 | 0.09× | 0 | 0 |
| regexp | fuzzy-hit | 0.95 | 8.25 | 0.11× | 0 | 0 |
| regexp | fuzzy-word | 0.89 | 8.69 | 0.10× | 0 | 0 |
| regexp | fuzzy-cjk | 0.81 | 18.90 | 0.04× | 0 | 0 |
| regexp | fuzzy-miss | 0.83 | 8.24 | 0.10× | 0 | 0 |
| regexp | regex-date | 9.25 | 20.69 | 0.45× | 1 | 1 |
| regexp | regex-email | 100.29 | 235.00 | 0.43× | 26 | 34 ⚠️ |
| regexp | regex-prefix | 0.69 | 0.67 | 1.03× | 124 | 124 |
| regexp | regex-miss | 0.88 | 28.63 | 0.03× | 0 | 0 |
| fuzzy | hit-english | 66.68 | 48.25 | 1.38× | 21 | 21 |
| fuzzy | hit-partial | 61.40 | 23.76 | 2.58× | 748 | 748 |
| fuzzy | hit-cjk | 60.36 | 20.97 | 2.88× | 178 | 178 |
| fuzzy | hit-code | 66.46 | 43.12 | 1.54× | 308 | 308 |
| fuzzy | hit-unicode | 60.21 | 18.89 | 3.19× | 107 | 107 |
| fuzzy | hit-casefold | 65.72 | 38.01 | 1.73× | 199 | 199 |
| fuzzy | hit-long-item | 67.19 | 48.18 | 1.39× | 537 | 537 |
| fuzzy | miss | 74.74 | 78.29 | 0.95× | 2 | 2 |
| fuzzy | fuzzy-hit | 61.58 | 23.92 | 2.57× | 426 | 426 |
| fuzzy | fuzzy-word | 63.10 | 24.32 | 2.59× | 154 | 154 |
| fuzzy | fuzzy-cjk | 59.46 | 17.80 | 3.34× | 189 | 189 |
| fuzzy | fuzzy-miss | 62.42 | 27.17 | 2.30× | 2 | 2 |
| fuzzy | regex-date | 67.60 | 47.21 | 1.43× | 17 | 17 |
| fuzzy | regex-email | 72.26 | 68.32 | 1.06× | 0 | 0 |
| fuzzy | regex-prefix | 62.77 | 28.07 | 2.24× | 165 | 165 |
| fuzzy | regex-miss | 67.65 | 47.10 | 1.44× | 0 | 0 |
| mixed | hit-english | 80.67 | 43.64 | 1.85× | 1 | 1 |
| mixed | hit-partial | 77.61 | 43.88 | 1.77× | 6 | 6 |
| mixed | hit-cjk | 77.94 | 78.38 | 0.99× | 71 | 71 |
| mixed | hit-code | 79.76 | 44.01 | 1.81× | 19 | 19 |
| mixed | hit-unicode | 76.55 | 43.93 | 1.74× | 1 | 1 |
| mixed | hit-casefold | 80.50 | 45.01 | 1.79× | 1 | 1 |
| mixed | hit-long-item | 150.21 | 103.23 | 1.46× | 537 | 537 |
| mixed | miss | 159.89 | 146.04 | 1.09× | 2 | 2 |
| mixed | fuzzy-hit | 139.75 | 85.90 | 1.63× | 426 | 426 |
| mixed | fuzzy-word | 141.28 | 86.64 | 1.63× | 154 | 154 |
| mixed | fuzzy-cjk | 138.36 | 133.25 | 1.04× | 189 | 189 |
| mixed | fuzzy-miss | 152.96 | 90.09 | 1.70× | 2 | 2 |
| mixed | regex-date | 90.07 | 50.53 | 1.78× | 17 | 17 |
| mixed | regex-email | 188.27 | 299.01 | 0.63× | 24 | 34 ⚠️ |
| mixed | regex-prefix | 84.91 | 47.75 | 1.78× | 124 | 124 |
| mixed | regex-miss | 156.89 | 134.66 | 1.17× | 0 | 0 |

### n = 10000

| 模式 | 查询 | Go ms | Swift ms | Go/Swift | Go 命中 | Swift 命中 |
| --- | --- | --- | --- | --- | --- | --- |
| exact | hit-english | 550.52 | 295.14 | 1.87× | 1 | 1 |
| exact | hit-partial | 553.86 | 273.03 | 2.03× | 10 | 10 |
| exact | hit-cjk | 561.88 | 506.04 | 1.11× | 367 | 367 |
| exact | hit-code | 544.91 | 290.40 | 1.88× | 92 | 92 |
| exact | hit-unicode | 592.26 | 323.87 | 1.83× | 1 | 1 |
| exact | hit-casefold | 539.06 | 281.44 | 1.92× | 1 | 1 |
| exact | hit-long-item | 604.56 | 252.33 | 2.40× | 1 | 1 |
| exact | miss | 563.97 | 256.20 | 2.20× | 0 | 0 |
| exact | fuzzy-hit | 595.00 | 267.84 | 2.22× | 0 | 0 |
| exact | fuzzy-word | 558.46 | 271.91 | 2.05× | 0 | 0 |
| exact | fuzzy-cjk | 611.85 | 429.63 | 1.42× | 0 | 0 |
| exact | fuzzy-miss | 578.16 | 255.52 | 2.26× | 0 | 0 |
| exact | regex-date | 587.78 | 289.33 | 2.03× | 109 | 109 |
| exact | regex-email | 675.77 | 291.22 | 2.32× | 0 | 0 |
| exact | regex-prefix | 564.63 | 247.01 | 2.29× | 0 | 0 |
| exact | regex-miss | 647.93 | 241.25 | 2.69× | 0 | 0 |
| regexp | hit-english | 5.91 | 49.87 | 0.12× | 1 | 1 |
| regexp | hit-partial | 7.56 | 52.49 | 0.14× | 10 | 10 |
| regexp | hit-cjk | 5.93 | 111.55 | 0.05× | 367 | 367 |
| regexp | hit-code | 6.45 | 53.19 | 0.12× | 92 | 92 |
| regexp | hit-unicode | 8.01 | 77.73 | 0.10× | 1 | 1 |
| regexp | hit-casefold | 5.72 | 45.26 | 0.13× | 0 | 0 |
| regexp | hit-long-item | 6.51 | 48.18 | 0.14× | 1 | 1 |
| regexp | miss | 5.84 | 47.89 | 0.12× | 0 | 0 |
| regexp | fuzzy-hit | 7.14 | 48.77 | 0.15× | 0 | 0 |
| regexp | fuzzy-word | 7.08 | 51.03 | 0.14× | 0 | 0 |
| regexp | fuzzy-cjk | 6.30 | 124.53 | 0.05× | 0 | 0 |
| regexp | fuzzy-miss | 5.95 | 50.88 | 0.12× | 0 | 0 |
| regexp | regex-date | 48.64 | 119.80 | 0.41× | 1 | 1 |
| regexp | regex-email | 100.25 | 1295.65 | 0.08× | 14 | 189 ⚠️ |
| regexp | regex-prefix | 4.72 | 4.26 | 1.11× | 556 | 556 |
| regexp | regex-miss | 6.13 | 154.38 | 0.04× | 0 | 0 |
| fuzzy | hit-english | 544.02 | 310.13 | 1.75× | 76 | 76 |
| fuzzy | hit-partial | 532.50 | 146.81 | 3.63× | 3687 | 3687 |
| fuzzy | hit-cjk | 462.91 | 140.40 | 3.30× | 963 | 963 |
| fuzzy | hit-code | 523.71 | 254.74 | 2.06× | 1430 | 1430 |
| fuzzy | hit-unicode | 457.42 | 107.26 | 4.26× | 439 | 439 |
| fuzzy | hit-casefold | 497.40 | 248.10 | 2.00× | 939 | 939 |
| fuzzy | hit-long-item | 457.64 | 347.24 | 1.32× | 2527 | 2527 |
| fuzzy | miss | 550.51 | 494.99 | 1.11× | 12 | 12 |
| fuzzy | fuzzy-hit | 500.72 | 150.22 | 3.33× | 2238 | 2238 |
| fuzzy | fuzzy-word | 450.13 | 150.20 | 3.00× | 846 | 846 |
| fuzzy | fuzzy-cjk | 393.48 | 107.51 | 3.66× | 955 | 955 |
| fuzzy | fuzzy-miss | 454.69 | 169.67 | 2.68× | 3 | 3 |
| fuzzy | regex-date | 493.65 | 306.72 | 1.61× | 109 | 109 |
| fuzzy | regex-email | 614.23 | 419.00 | 1.47× | 0 | 0 |
| fuzzy | regex-prefix | 444.44 | 205.21 | 2.17× | 810 | 810 |
| fuzzy | regex-miss | 542.00 | 323.02 | 1.68× | 0 | 0 |
| mixed | hit-english | 581.10 | 257.30 | 2.26× | 1 | 1 |
| mixed | hit-partial | 548.64 | 262.38 | 2.09× | 10 | 10 |
| mixed | hit-cjk | 560.01 | 437.55 | 1.28× | 367 | 367 |
| mixed | hit-code | 559.04 | 275.41 | 2.03× | 92 | 92 |
| mixed | hit-unicode | 550.80 | 270.05 | 2.04× | 1 | 1 |
| mixed | hit-casefold | 595.79 | 304.61 | 1.96× | 1 | 1 |
| mixed | hit-long-item | 570.29 | 271.85 | 2.10× | 1 | 1 |
| mixed | miss | 1417.82 | 816.72 | 1.74× | 12 | 12 |
| mixed | fuzzy-hit | 1016.08 | 444.26 | 2.29× | 2238 | 2238 |
| mixed | fuzzy-word | 1079.83 | 484.76 | 2.23× | 846 | 846 |
| mixed | fuzzy-cjk | 985.95 | 692.95 | 1.42× | 955 | 955 |
| mixed | fuzzy-miss | 1011.27 | 489.66 | 2.07× | 3 | 3 |
| mixed | regex-date | 584.31 | 270.46 | 2.16× | 109 | 109 |
| mixed | regex-email | 773.88 | 1669.65 | 0.46× | 12 | 189 ⚠️ |
| mixed | regex-prefix | 529.38 | 280.03 | 1.89× | 556 | 556 |
| mixed | regex-miss | 1104.62 | 716.61 | 1.54× | 0 | 0 |

## 2. 应用级指标

**原版 Maccy（真实 200 条 / 1.8GB 历史）**

| 指标 | 样本 | 中位数 |
| --- | --- | --- |
| 存储就绪 | 1807 255 250 356 282 | 282 ms |
| CPU 安静 | 2855 1011 1082 925 963 | 1011 ms |
| RSS | 269664 218336 340272 258336 299232 | 269664 KB |
| phys_footprint | 265216 266240 169984 211968 228352 | 228352 KB |
| 空闲 CPU% | 0.0 0.0 0.0 0.0 0.0 | 0 % |

**Go 版（合成 200 条历史）**

| 指标 | 样本 | 中位数 |
| --- | --- | --- |
| socket 就绪 | 94 99 97 95 93 | 95 ms |
| CPU 安静 | 576 506 508 500 455 | 506 ms |
| RSS | 83936 85392 83808 83504 83328 | 83808 KB |
| phys_footprint | 30720 32768 30720 30720 30720 | 30720 KB |
| 空闲 CPU% | 0.25 0.12 0.12 0.13 0.12 | 0 % |

**剪贴板捕获延迟（ms，写入 → 历史可见）**

- upstream: median=280  min=86.0  max=539.0  n=25
- go: median=264  min=91.0  max=580.0  n=25

注：两侧默认轮询间隔均为 0.5s，故延迟呈 0-500ms 均匀分布，中位数 ≈ 250ms 为理论值。

**Go 版 UI 搜索延迟**

```
query=needle-alpha-42 ui_settle_ms=53
query=needle ui_settle_ms=48
query=剪贴板历史 ui_settle_ms=51
```
含 automation IPC 与渲染轮询开销，仅作参考量级。

**体积**

```
upstream binary: 3794160 bytes
upstream bundle:  10M
go binary:       16121762 bytes
go bundle:        15M
```

