# Maccy-MyGo

基于 [MyGo](https://github.com/egoist/mygo)，使用 Go 重写的 [Maccy](https://github.com/p0deje/Maccy) 剪贴板管理器。

![预览](docs/screenshots/preview.png)

## 下载

前往 [Releases](https://github.com/binyouo/Maccy-MyGo/releases) 下载 macOS / Linux / Windows 版本。发布包未经签名公证。

## 性能

与原版 Maccy 2.7.1 同机对比（macOS arm64）：

| 指标 | 原版 | 本版 |
| --- | --- | --- |
| 启动至空闲 | ~846 ms | ~481 ms |
| 空闲内存 (phys_footprint) | ~123 MB | ~31 MB |
| 搜索 200 条（exact / fuzzy） | 6.7 / 4.4 ms | 0.4 / 1.5 ms |
| 剪贴板捕获延迟 | ~250 ms | ~300 ms（同为 0.5s 轮询，属相位差） |
| 体积 | 10 MB | 15 MB（单文件静态链接，无框架依赖） |

## 开发

需要 Go 1.27.3+。

```sh
CGO_ENABLED=0 go run .
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo@v0.7.4 build
```

## 许可

[MIT](LICENSE) · [第三方许可](THIRD_PARTY_NOTICES.md)
