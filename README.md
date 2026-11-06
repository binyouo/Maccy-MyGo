# Maccy-MyGo

基于 [MyGo](https://github.com/egoist/mygo)，使用 Go 重写的 [Maccy](https://github.com/p0deje/Maccy) 剪贴板管理器。

![预览](docs/screenshots/preview.png)

## 下载

前往 [Releases](https://github.com/binyouo/Maccy-MyGo/releases) 下载 macOS / Linux / Windows 版本。发布包未经签名公证。

## 性能

与原版 Maccy 2.8.0 同机对比（macOS arm64，200 条文本历史）：

| 指标 | 原版 Maccy | MyGo |
| --- | ---: | ---: |
| 启动至空闲 | 1212ms | **563ms** |
| 空闲内存 | 42.1MiB | **28.4MiB** |
| 精确搜索 | 2.43ms | **0.34ms** |
| 模糊搜索 | 3.17ms | **0.97ms** |
| 空闲 CPU | 0.058% | **0.032%** |
| 空闲唤醒次数/秒 | 3.37 | **2.37** |

启动耗时约减半，常驻内存减少约 **33%**，精确搜索耗时约为原版的 **1/7**。同时支持 macOS、Linux 和 Windows。

搜索为算法基准；启动数据采自轮询优化前，空闲数据已更新至优化后。

## 开发

需要 Go 1.27.3+。macOS 轮询桥接已附带通用二进制；修改其原生源码后，用 `./tools/build-clipboard-poll.sh` 重建（需要 Command Line Tools）。

```sh
CGO_ENABLED=0 go run .
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo@v0.7.4 build
```

## 许可

[MIT](LICENSE) · [第三方许可](THIRD_PARTY_NOTICES.md)
