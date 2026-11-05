# Maccy-MyGo

一个使用 [MyGo](https://github.com/egoist/mygo) 桌面框架与 Go 编写的跨平台剪贴板管理器，
这是 macOS [Maccy](https://github.com/p0deje/Maccy) 的非官方重写，
与原项目及其作者没有官方关联。界面使用 MyGo 的
原生 UI（`ui` 包）绘制：没有 HTML、没有 JavaScript、没有 webview，单个 Go 二进制即可运行，
同一份代码可编译到 macOS、Windows 与 Linux。

> 与 Maccy 源码的逐文件覆盖审计见 [AUDIT.md](AUDIT.md)。

![Maccy MyGo 预览](docs/screenshots/preview.png)

## 下载

macOS Apple Silicon 版本见 [Releases](https://github.com/binyouo/Maccy-MyGo/releases)。当前发布包采用 ad hoc 签名，尚未经过 Apple 公证。Windows/Linux 已完成编译检查，尚未做桌面实机验收。

## 来源与许可

- 交互及功能参考 [Maccy](https://github.com/p0deje/Maccy)，对照提交为 [`f0b571e`](https://github.com/p0deje/Maccy/commit/f0b571e32e4031d7b91ba811f50d0f33e8e5548b)。保留了原版菜单栏模板资源，并移植了部分行为和算法。
- 原生桌面框架：[MyGo](https://github.com/egoist/mygo) `v0.7.4`。
- 模糊搜索移植自 [Fuse-Swift](https://github.com/krisk/fuse-swift) `1.4.0`；部分工具条图形沿用 Lucide/Feather 图标。
- 本项目使用 [MIT 许可证](LICENSE)。原作者版权和依赖许可见 [第三方说明](THIRD_PARTY_NOTICES.md) 与随包分发的 `resources/licenses/`。

图中的历史记录均为合成示例。

## 功能对照

| Maccy（Swift） | 本项目（Go + MyGo） |
| --- | --- |
| 菜单栏图标 `AppDelegate` | `internal/app`（`mygo.NewTray`） |
| 剪贴板轮询 `Clipboard.swift` | `internal/clipboard/watcher.go` |
| 历史记录 `History.swift` / `HistoryItem` | `internal/model`、`internal/app/history.go` |
| 去重与次数合并 `History.add` | `App.addItem`（按内容指纹合并，累加 `numberOfCopies`） |
| 搜索 `Search.swift`（exact/fuzzy/regexp/mixed） | `internal/search` |
| 排序 `Sorter.swift`（最后/首次复制、复制次数） | `orderItems` |
| 置顶 `PinManager`（随机可用键、置顶顺序） | `togglePin`、`availablePins`、`pinOrder` |
| 弹出面板 `FloatingPanel` / `ContentView` | `internal/app/popup.go`（`mygo.WindowOptions{Panel:true}`） |
| 搜索框、列表、页脚 `HeaderView/HistoryListView/FooterView` | `header`、`listView`、`footer` |
| 预览侧栏 `SlideoutView` / `PreviewItemView` | `preview`（Ctrl+Space 切换） |
| 设置窗口六个面板 `Settings/*` | `internal/app/settings_ui.go`（通用/存储/外观/置顶项/忽略/高级） |
| 全局快捷键（默认 Cmd/Shift+C） | `mygo.GlobalShortcut` |
| 复制 / 粘贴 / 去格式粘贴 | `activateItem`、`itemAction`、`clipboard.Paste` |
| 忽略类型 / 忽略正则 / 忽略应用 | `internal/clipboard/capture.go` |
| 持久化 `Storage.sqlite` | `internal/model/model.go`（JSON 索引 + 内容寻址 blob） |

## 目录结构

```
main.go                     应用入口：单实例锁、菜单栏应用、启动
internal/config/            偏好设置（config.json）与默认值
internal/model/             历史条目模型与磁盘存储（history.json + blobs/）
internal/clipboard/         剪贴板轮询、捕获、还原、粘贴
internal/search/            搜索算法（精确 / 模糊 / 正则 / 混合）
internal/app/               应用状态、弹出面板 UI、偏好窗口、托盘与快捷键
resources/icon.png          应用图标
resources/ocr              macOS 图片 OCR 助手（Vision，通用二进制）
tools/ocr.swift            OCR 助手源码
mygo.json                   打包配置（名称、标识符、版本、图标）
```

## 运行与构建

需要 Go 1.27.3+（MyGo 要求）。本项目不使用 cgo。

```sh
# 直接运行（开发）
CGO_ENABLED=0 go run .

# 运行测试
CGO_ENABLED=0 go test ./...

# 交叉编译检查（MyGo 纯 Go，无需 cgo）
CGO_ENABLED=0 GOOS=linux  go build ./...
CGO_ENABLED=0 GOOS=windows go build ./...

# 打包为原生应用（macOS .app，Windows 安装包，Linux deb/PKGBUILD）
CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo@v0.7.4 build
```

打包产物位于 `build/<os>-<arch>/`。

## 使用

- 默认全局快捷键 <kbd>Cmd/Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>C</kbd> 打开面板（也可点击菜单栏图标）。
- 输入即搜索；<kbd>↑</kbd>/<kbd>↓</kbd>、<kbd>Home</kbd>/<kbd>End</kbd>、<kbd>PageUp</kbd>/<kbd>PageDown</kbd> 移动选择。
- <kbd>Return</kbd> 复制；<kbd>Option</kbd>+<kbd>Return</kbd> 复制并粘贴；<kbd>Option</kbd>+<kbd>Shift</kbd>+<kbd>Return</kbd> 去格式粘贴。
- <kbd>Option</kbd>+<kbd>P</kbd> 置顶/取消置顶；<kbd>Option</kbd>+<kbd>Delete</kbd> 删除条目。
- <kbd>Ctrl</kbd>+<kbd>Space</kbd> 切换预览；<kbd>Cmd/Ctrl</kbd>+<kbd>,</kbd> 打开设置。
- <kbd>Cmd/Ctrl</kbd>+<kbd>Option</kbd>+<kbd>Delete</kbd> 清空未置顶条目，加上 <kbd>Shift</kbd> 清空全部。

> 粘贴是通过模拟系统粘贴键实现的，macOS 需要在「系统设置 → 隐私与安全性 → 辅助功能」
> 中授权本应用；未授权时仍会把条目复制到剪贴板。

## 数据位置

`App.Path(PathUserData)`，在 macOS 为 `~/Library/Application Support/Maccy MyGo`：
`config.json` 保存设置，`history.json` 保存历史索引，`blobs/` 保存各格式内容。

## 开发用环境变量

- `MACCY_WINDOW=1`：把弹出面板渲染成普通窗口，便于调试。
- `MACCY_PREVIEW=1`：配合上者显示预览侧栏。
- `MACCY_PREFS=1`、`MACCY_PREFSTAB=N`：直接打开设置窗口并选择分页。
- `MACCY_QUERY=...`：预置搜索词。
- `MACCY_APPICONS=1`：打开"显示应用图标"。
- `MACCY_EDIT=1`：打开条目编辑器。
- `MACCY_AUTOSHOW=1`：启动后自动打开面板。
- `MACCY_KEEPOPEN=1`：面板失去焦点时不自动关闭（截图用）。
- `MACCY_NAME=...`：覆盖应用名（独立的单实例锁和数据目录，便于并行测试）。
- `MACCY_TEST_SCREEN=N`：将开发窗口、设置、关于和弹出面板固定到第 N 块屏幕（从 1 开始）；屏幕不存在或选中主屏时拒绝启动，避免副屏测试误落到主屏。
- `MACCY_TEST_EDGE=left/right`：配合上者在副屏边缘打开窗口，用于检查连续开合预览和切换设置页后的原点恢复。
- `MYGO_AUTOMATION=socket路径`：开启自动化套接字，供 `mygo-automation` CLI 驱动界面。

## 界面语言与性能（最新）

- **弹出面板按 Maccy 重排**：页头只有标题/圆角搜索框（放大镜 + 清除钮）与预览切换，条目操作（复制识别文本/置顶/编辑/删除）移到预览侧栏顶部工具条；列表为 22pt 行高、4pt 圆角、`#0A84FF`×0.8 选中色，应用图标 + 缩略图/色块 + 标题 + 行尾快捷键提示；置顶块与其余条目间以分隔线隔开；页脚是纵向的 清除/偏好设置…/关于/退出 行，按住 ⇧（⌘⌥⇧ 集合内）时「清除」换作「全部清除」，悬停页脚会收起预览。
- **行尾快捷键提示动态切换**：与 Maccy `KeyShortcut.create` 一致为每个可激活条目准备三个变体（⌘x / ⌥x / ⇧⌘x 或 ⌥⇧x，取决于 pasteByDefault），按住修饰键实时切换显示，且激活逻辑与显示一致；置顶字母键与 ⌘1–⌘9 同一规则。
- **界面全中文**：弹窗（搜索/清除/偏好设置…/关于/退出、右键菜单、编辑器）、五个设置页、关于窗口、托盘菜单均为简体中文。
- **设置窗口（参考 Maccy Settings）**：顶部为图标与标题组成的六个页签（通用/存储/外观/置顶项/忽略/高级），支持方向键切换；内容区使用右对齐标签列、左对齐控件与组间分隔线。外观页使用 MyGo 原生复选框、数字输入及基于原生分段控件绘制的选择器，跟随系统深浅色；包含菜单栏图标、搜索框显示时机、自动预览与预览延迟等选项。600pt 窄窗口中位置重置按钮另起一行，超过三块显示器时使用下拉选择。
- **设置功能补齐**：快捷键改为“点击后按下组合键”的录制控件（显示 ⌘⇧C 等键帽，可恢复默认）；忽略页的应用 / 剪贴板类型 / 正则为可增删的列表行，支持恢复默认、清空与正则合法性校验；外观页「上次位置」可重置，并在多次启动间记住上次弹窗位置；高级页新增「清空前确认」。
- **⌘1–⌘9 快捷选择**：未置顶条目显示 ⌘1…⌘9，按下即复制对应条目（带修饰键时按 Maccy 的复制/粘贴规则执行）。
- **页脚快捷键提示**：清除 ⌥⌘⌫、偏好设置 ⌘,、退出 ⌘Q。
- **性能**：应用图标、图片缩略图改为后台异步加载并缓存；弹窗仅在尺寸变化时重设大小，避免无效 resize；列表虚拟化，仅构建可见行。
- **鼠标/触控板滚动**：列表使用 MyGo 的滚动容器并设置 `KeepOffset`（按像素偏移锚定，而非按顶部行），变高行（图片）滚动时不再相对光标跳动；弹出高度按真实行高（图片行更高）计算。
- **悬停与点击**：鼠标悬停高亮并同步移动选中行；行内子元素设为 PassThrough，点击行任意位置都能选中/复制（修复了子元素吞掉点击的问题）。

## 已补齐的 Maccy 功能（持续更新）

- 每行来源应用图标：macOS 下通过 `lsappinfo` 取得前台应用 Bundle ID，并用
  `mdfind` + `sips` 提取并缓存应用图标（`showApplicationIcons`）。
- 条目编辑器：重命名别名、修改纯文本内容、改置顶键（工具条 ✎ / 右键 Edit…）。
- 行右键菜单：复制 / 粘贴 / 去格式粘贴 / 置顶 / 编辑 / 删除。
- 置顶项拖拽排序（`ListState.Reorder`，仅在置顶区且置顶在顶部时生效）。
- 自动预览：选中项停留 `previewDelay` 后自动展开预览侧栏（可在设置中关闭）。
- 搜索框显隐（`showSearch` / `searchVisibility`），隐藏时仍可键盘输入过滤。
- 忽略应用名单（按前台应用 Bundle ID，含"仅保留列出的应用"模式）。
- 清空历史时可选同时清空系统剪贴板。
- 跟随登录启动（macOS LaunchAgent）。
- 记住上次弹出位置（Appearance → Popup at → Last position）。
- Windows 使用 `SendInput` 模拟 Ctrl+V 粘贴；Linux 使用 `xdotool`。
- 多显示器"在指定屏幕打开"选择（Appearance → Screen）。
- 忽略应用的图形化选择器（Ignore → Choose application…，自动读取 Bundle ID）。
- 图片 OCR 生成标题：macOS 使用随包分发的 Vision 助手 `resources/ocr`；复制图片后先立即入库，再在后台识别并更新标题。
- 自动更新：集成 MyGo 原生 updater 插件（Sparkle 风格窗口），应用菜单与菜单栏均有「Check for Updates…」，设置里可开关后台自动检查、立即检查。
- 页脚键盘导航：↓ 越过最后一项进入页脚，↑/↓ 在 Clear/Preferences/About/Quit 间移动，Enter 执行，↑ 从首项返回列表。
- 匹配高亮样式：Bold / Color / Italic / Underline 均按文字片段绘制，Color 使用黄色背景黑字；完整条目标题保留为无障碍名称。
- About 窗口显示应用图标。
- 复制通知：新条目写入时用系统通知提示（Advanced 可关闭「Notifications」）。
- 自动化回归测试：`internal/app/popup_test.go` 使用 `ui.Tester` 覆盖搜索过滤、键盘导航、页脚导航、预览切换、条目编辑取消、工具条置顶、快捷键解析。
- 可配置快捷键：设置 General 可编辑 Open / Pin / Delete / Show preview。Pin 在弹窗打开时通过临时全局热键捕获（等价于 Maccy 的本地按键监听），Delete/Preview 由输入层按键处理。

## 尚未完全对齐的部分

- 真实更新发布渠道：已接入 MyGo updater（菜单/自动检查/更新窗口），但需在 `mygo.json` 配置
  `updates.publicKey` 与 `updates.github`（或 url），并用 `MYGO_UPDATER_PRIVATE_KEY` 签名发布后才可实际推送更新。
- App Intents / macOS 快捷指令、自定义通知音效、完整键盘布局转换尚未移植；跨平台接口与真机验证边界见 [AUDIT.md](AUDIT.md)。
- 多选/粘贴栈在参考 Swift 版本中被常量 `multiSelectionEnabled=false` 禁用，当前不额外启用。

## 图片 OCR 助手

`tools/ocr.swift` 使用 macOS Vision 生成图片文字；构建通用二进制：

```sh
/usr/bin/swiftc -O -target arm64-apple-macosx13.0 tools/ocr.swift -o /tmp/ocr-arm64
/usr/bin/swiftc -O -target x86_64-apple-macosx13.0 tools/ocr.swift -o /tmp/ocr-x86_64
lipo -create -output resources/ocr /tmp/ocr-arm64 /tmp/ocr-x86_64
chmod +x resources/ocr
```

运行时由 `internal/clipboard/ocr_darwin.go` 调用；其它平台跳过 OCR。

## 补充对齐（2026-11-05）

- Option / Option+Shift 点击历史行的粘贴动作；菜单栏 Option 点击暂停、Option+Shift 忽略下一次。
- macOS 主快捷键保持修饰键连按可循环选择，松键确认；编辑时失焦保留草稿。
- 原生窗口边缘调整和可拖动预览分栏（左右展开均支持，最小 200pt，分栏支持键盘）；保存栏宽和相对屏幕位置。
- 原版四种菜单栏图标、重启恢复、暂停外观；动态工具条快捷键提示与实际搜索高亮。
- 剪贴板按格式读取；索引成功保存后回收无引用内容，保护捕获交接和保存失败时的数据。

源码依据、测试与剩余差异以 [AUDIT.md](AUDIT.md) 为准。
