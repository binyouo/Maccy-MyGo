# Maccy-MyGo

基于 [MyGo](https://github.com/egoist/mygo)，使用 Go 重写的 [Maccy](https://github.com/p0deje/Maccy) 剪贴板管理器。

![预览](docs/screenshots/preview.png)

## 下载

前往 [Releases](https://github.com/binyouo/Maccy-MyGo/releases) 下载 macOS Apple Silicon 版本。当前发布包尚未经过 Apple 公证。

## 开发

需要 Go 1.27.3+。

```sh
CGO_ENABLED=0 go run .
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo@v0.7.4 build
```

## 许可

[MIT](LICENSE) · [第三方许可](THIRD_PARTY_NOTICES.md)
