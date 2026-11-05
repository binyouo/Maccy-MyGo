# tools/bench — Maccy (Swift) vs Maccy-MyGo (Go) 性能对比

对比对象：

- **原版**：`/Applications/Maccy.app`（Swift/AppKit，Maccy 2.x，真实 1.8GB 历史库）
- **现版**：本仓库 Go 重写版（`internal/search` + `internal/model` + MyGo 原生 UI）

## 组成

| 文件 | 作用 |
| --- | --- |
| `bench.sh` | 一键编排：构建 → 微基准 → 启动/内存/CPU → 剪贴板捕获延迟 → UI 搜索延迟 → 恢复现场 |
| `gen/` | 生成确定性合成语料 `corpus.json`（10k 条标题，中英混排 + 长文本 + 针尖条目）与 `queries.json`（16 条查询） |
| `searchgo/` | Go 侧搜索微基准：直接调 `internal/search.Run`，输出 min/median/p95 ns |
| `swiftsearch/` | Swift 对照基准：逐行复刻 Maccy `Search.swift`（NSString 比较 + NSRegularExpression + Fuse 1.4.0），同语料同查询 |
| `seed/` | 按现版磁盘格式（history.json + blobs）生成 200 条合成历史供 bench 实例使用 |
| `fdprobe.py` | libproc fd 探针，~1ms 检测进程是否打开某文件（原版 SwiftData 就绪信号） |

## 运行

```sh
tools/bench/bench.sh          # 完整：5 次启动、20 次捕获、微基准 25 reps
tools/bench/bench.sh quick    # 快速：3 次启动、10 次捕获、8 reps
```

结果写入 `/tmp/maccy-bench/out/`（JSON + 汇总文本），并汇总为 `RESULTS.md`。

前置依赖：macOS、Go、swiftc、python3、`/tmp/mygo-automation`（mygo-automation CLI；
可用 `go run github.com/egoist/mygo/...` 构建或 `AUTOMATION=/path` 覆盖）。
原版 Fuse 源码默认取 `/tmp/mygomaccy-fuse-reference/Fuse/Classes`，
不存在时自动 `git clone krisk/fuse-swift` 并 checkout 到 Maccy 锁定的
`26ba868`（1.4.0）；可用 `FUSE_DIR=` 覆盖。

## 度量口径

- **微基准**：同一 `corpus.json`/`queries.json` 喂给两个 harness，计 per-query
  全表扫描耗时（warmup 后 reps 次取 min/median/p95）。Swift 端复刻了 Maccy
  的模式分支、Fuse threshold=0.7、5000 字符截断与 score 排序。
- **启动**：`posix_spawn` → ①存储打开（原版 = fdprobe 见 Storage.sqlite；
  Go = automation socket 就绪）②CPU 连续 3×60ms < 2%（t_quiet）。后者口径对两边一致，为主指标。
- **空闲**：settle 后 RSS + `footprint`（phys_footprint）；8s 窗口 CPU 时间增量。
- **捕获延迟**：`pbcopy` 写入 marker → 原版 sqlite 出现该标题 / Go `history.json`
  出现该文本的耗时；两 app 同刻轮询（均默认 0.5s interval）。
- **UI 延迟**：automation `type` 查询 → `wait --text` 命中条目可见（含 IPC/轮询开销，
  仅作 Go 端参考值；原版无自动化接口）。

## 安全说明

- 原版真实历史 `Storage.sqlite*` 在测试前复制到 `/tmp/maccy-bench/backup/`，
  脚本退出时（trap EXIT）原样恢复。
- 正在运行的现版实例会被临时退出并在结束后重新 `open`。
- 系统剪贴板文本会先 `pbpaste` 备份、结束 `pbcopy` 还原（仅文本）。
- bench 实例使用 `MACCY_NAME=MaccyBench` 独立存储目录，不触碰真实数据。

## 已知局限

- 原版无自动化接口：其 UI 层延迟以微基准 + 启动/内存代理，不测量渲染耗时。
- `pbcopy` 写入源记为终端应用，与真实用户复制路径略有差异。
- 空闲 CPU 受其他进程调度影响，±0.5% 属噪声。
