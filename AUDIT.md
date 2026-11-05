# Maccy 复刻覆盖审计

本文件逐组对照 Maccy 源码（`reference-Maccy/Maccy`）说明本项目（Go + MyGo）的实现情况。

本轮实际调用链对照基于 `reference-Maccy` 提交 `f0b571e32e4031d7b91ba811f50d0f33e8e5548b`（`2.8.0-1-gf0b571e`）。本项目运行依赖为 MyGo `v0.7.4`；目录中的 MyGo 参考源码不作为运行版本的替代证据。表中的“已实现”表示对应行为已有实现，下方单独列出已经验证的同步项和仍有差异的边界。

图例：✅ 已实现 · ◐ 近似/部分 · ➖ 不适用（Swift/AppKit 实现细节） · ⛔ 平台受限/上游默认关闭

## 应用骨架

| Maccy | 本项目 | 状态 |
| --- | --- | --- |
| MaccyApp.swift / AppDelegate.swift | main.go、internal/app/app.go | ✅ 菜单栏应用、单实例、托盘、菜单 |
| FloatingPanel.swift | internal/app/app.go（`WindowOptions{Panel:true}`） | ✅ 非激活浮窗、失焦关闭、可移动 |
| GlobalHotKey.swift / KeyboardShortcuts | `mygo.GlobalShortcut` + 可配置快捷键 | ✅ |
| MenuIcon.swift | internal/app/icons.go（trayIconFor） | ✅ 四种原版模板资源；启动恢复选中图标，暂停/停用存储时降低透明度 |
| Notifier.swift | internal/app/history.go（`mygo.NewNotification`） | ✅ 新复制通知（可关） |
| SoftwareUpdater.swift | MyGo `plugins/updater/native` | ✅ 接好更新窗口/菜单/自动检查；发布渠道需配置 |
| Accessibility.swift | 依赖系统辅助功能 + osascript 粘贴 | ➖ 当前 Swift 快照的 Accessibility.check() 也是空操作；系统仍要求辅助功能权限 |
| AppStoreReview.swift / numberOfUsages | — | ➖ 商店评分 |
| VoiceOver.swift | 原生 UI 自带无障碍；关键控件加 Label | ◐ |

## 数据与剪贴板

| Maccy | 本项目 | 状态 |
| --- | --- | --- |
| Clipboard.swift | internal/clipboard/watcher.go、capture.go、restore.go | ✅ 轮询、类型过滤、去重、忽略规则、还原 |
| Models/HistoryItem.swift | internal/model/model.go | ✅ 标题生成、预览文本、文件/图片/富文本 |
| Models/HistoryItemContent.swift | internal/model（Content） | ✅ |
| Models/SizedString.swift | clipboard/capture.go（shorten） | ✅ |
| Storage.swift（SwiftData/SQLite） | internal/model/model.go（JSON + 内容寻址 blob） | ◐ 原子索引/内容写入，成功保存后回收；失败保护旧索引与待保存内容，仍非 SwiftData 事务 |
| HistoryItemAction.swift / KeyChord.swift | internal/app/actions.go、shortcuts.go | ✅ 修饰键动作、可配置快捷键解析 |
| HighlightMatch.swift | popup_highlight.go（四种分段渲染） | ✅ 四种样式 |
| Throttler.swift | 即时重建 | ➖ |
| KeyboardLayout.swift | — | ➖ 由 MyGo/系统处理键位 |
| PasteStack.swift | — | ➖ Swift 常量 `multiSelectionEnabled=false`；与原版当前可用功能一致 |
| Selection.swift（多选） | — | ➖ 同上 |
| ColorImage.swift | popup.go（hexColor 色块） | ✅ |
| ApplicationImage(+Cache).swift | appicons*.go | ✅ macOS 提取/缓存；其它平台无图标 |
| Extensions/*.swift | 对应 Go 标准库/自定义函数 | ➖ |

## 历史与搜索

| Maccy | 本项目 | 状态 |
| --- | --- | --- |
| Observables/History.swift | internal/app/history.go | ✅ 加载、去重合并、裁剪、清空、排序、搜索联动 |
| Observables/HistoryItemDecorator.swift | popup.go（行渲染/高亮） | ✅ |
| Observables/PinManager.swift | history.go（availablePins/togglePin/movePin） | ✅ 动态保留操作键、随机分配、无键置顶、顺序、拖拽排序 |
| Observables/NavigationManager.swift | popup.go（选择/键盘/页脚导航/滚动） | ✅ 列表与页脚导航；多选扩展省略 |
| Observables/AppState.swift / Popup.swift | app.go | ✅ 状态、开合、快捷键生态 |
| Observables/Footer(.Item).swift | popup.go（footerActions） | ✅ |
| Observables/SlideoutController.swift | popup.go（previewOpen/自动预览/宽度） | ✅ 预览侧栏与自动展开 |
| Search.swift | internal/search/ | ✅ exact/fuzzy/mixed；正则方言边界见下文 |
| Sorter.swift | history.go（orderItems） | ✅ |
| ConcatenatedCollection / ItemsProtocol | 用切片组合 | ➖ |

## 视图

| Maccy | 本项目 | 状态 |
| --- | --- | --- |
| ContentView / VisualEffectView | popup.go（Vibrancy） | ✅ |
| HeaderView / ListHeaderView / SearchFieldView | popup.go（header + searchField） | ✅ 标题/圆角搜索框/清除钮/预览切换；可显隐、可聚焦 |
| ToolbarView | popup.go previewToolbar（预览面板顶部） | ✅ 识别文本/置顶/编辑/删除 |
| HistoryListView / HistoryItemView / ListItemView | popup.go（List + row） | ✅ 22pt 行高、4pt 圆角、选中色 0.8 透明度、应用图标、缩略图/色块、动态快捷键提示 |
| KeyboardShortcutView（行尾提示） | popup.go rowHint | ✅ ⌘/⌥/⇧⌘(或⌥⇧) 变体随修饰键切换，isVisible 规则一致 |
| KeyShortcut.create 三变体 | actions.go itemAction + history.go shortcuts | ✅ 激活与显示一致 |
| PinsView / ReorderableContainer | ListState.Reorder（置顶区） | ✅ 拖拽排序；置顶块以分隔线隔开 |
| FooterView / FooterItemView / ConfirmationView | popup.go footer + mygo.Dialog 确认 | ✅ 纵向行、键盘可达、清空确认 |
| Footer 清除/全部清除切换 | footer.footerClearAll（Shift∈⌘⌥⇧ 内判定） | ✅ 与 clearAllModifiersPressed 等价；悬停页脚收起预览 |
| ItemEditorView | internal/app/editor.go | ✅ 别名/内容/置顶键 |
| PreviewItemView / LargeTextView / AsyncView | popup.go preview | ✅ 文本/圆角图片/元数据（应用、尺寸、大小、首末复制、次数） |
| SlideoutView / SlideoutContentView | popup.go 两栏布局 + Divider | ✅ 顶部工具条 + 内容 + 底部元数据块 |
| AppImageView / HeightReader / HoverSelection / MouseMoved | MyGo 内置布局/悬停 | ➖ |

## 设置

| Maccy | 本项目 | 状态 |
| --- | --- | --- |
| GeneralSettingsPane | settings_ui.go generalPrefs | ✅ 更新/登录/快捷键录制/搜索/粘贴/系统通知入口 |
| StorageSettingsPane | storagePrefs | ✅ 类型开关/容量/排序/磁盘占用 |
| AppearanceSettingsPane | appearancePrefs | ✅ 位置/屏幕/置顶/图片高/预览延迟/高亮/显隐/菜单栏图标/搜索框显示时机/上次位置重置 |
| IgnoreSettingsPane（Apps/Types/Regexps） | ignorePrefs + 列表编辑 + 选择器 | ✅ 应用 / 剪贴板类型 / 正则三个子标签，可增删/恢复默认/正则校验 |
| AdvancedSettingsPane | advancedPrefs | ✅ 通知、暂停记录、退出清空、清空系统剪贴板、清空前确认、立即清空 |
| Settings 分区布局 | ui.Form / ui.Field / ui.Divider | ✅ 右对齐标签列 + 左对齐控件 + 细线分隔，无卡片 |
| Settings.PaneIdentifier+Panes | 顶部工具栏图标页签 settingsTab | ✅ 图标+标题页签，选中项用强调色 |

## 系统集成

| Maccy | 本项目 | 状态 |
| --- | --- | --- |
| Intents/*（App Intents / 快捷指令） | — | ⛔ 需 Swift AppIntents，MyGo/Go 不可直接实现 |
| Extensions/NSWorkspace+ApplicationName | frontmost_darwin.go（lsappinfo） | ✅ |
| NSScreen+ForPopup / PopupPosition.swift | app.go（popupOrigin/targetWorkArea） | ✅ 含多屏选择 |
| NSSound+Named | 通知由系统处理 | ◐ 无自定义音效 |

## 已知差异（有意为之）

1. **App Intents/Shortcuts**：Maccy 提供 macOS 快捷指令动作；Go/MyGo 无法直接声明 App Intents，标记为平台受限。
2. **粘贴栈/多选**：参考快照用常量 `multiSelectionEnabled=false` 禁用，此处不额外启用未开放功能。
3. **更新发布**：更新能力已接入，实际推送需配置密钥与 releases。
4. **高亮斜体/下划线**：通过分段文本实现，效果与 Maccy 一致；极长标题用裁剪代替中间省略。

## 2026-11-05 实际逻辑同步

本轮以 Swift 源码、原版测试及真实原生窗口为证据，修正了此前覆盖表未能体现的行为差异。

| 范围 | 已同步行为 | 主要回归证据 |
| --- | --- | --- |
| 捕获与忽略 | 敏感/忽略类型拒收整次复制，逐 pasteboard item 过滤空白与正则，排除动态表示及浏览器/Word 误导表示 | `internal/clipboard/parity_test.go` |
| 还原与去格式 | 缺少纯文本时保留图片/富文本，多 item 重复 MIME 不再 panic，补齐 Maccy/source 元数据 | `internal/clipboard/parity_test.go` |
| 复制序号 | macOS `NSPasteboard.changeCount`、Windows `GetClipboardSequenceNumber`；相同内容重复复制可计数，暂停时无需读取内容 | `change_count_darwin_test.go`、watcher 回归 |
| Universal Clipboard | 辅助 fileURL 不覆盖文本或真实图片；remote JPEG 文件用于预览/OCR，保留原始还原表示 | 临时文件协议 fixture，`clipboard.ImageData` |
| 去重与格式转换 | 超集去重、保留别名/富格式/来源/首次时间，ModifiedType 与本次运行复制序号关联；删除时清理会话映射 | `internal/app/history_parity_test.go` |
| 容量与排序 | 按当前排序裁剪，给刚复制条目保留位置；启动时执行容量限制；置顶与普通历史分别搜索 | `history_parity_test.go`、`interaction_parity_test.go` |
| 置顶 | 随机分配并排除操作键，键耗尽仍置顶；空键独立排序、不分配数字快捷键；拖拽目的位置和改键原位更新 | `history_parity_test.go` |
| 精确与模糊搜索 | Unicode 大小写与规范等价、原始高亮范围、连续 ZWJ/国旗；移植原版锁定的 Fuse 1.4.0 Bitap 与评分 | `internal/search/parity_test.go`，270 条真实 Swift Fuse 差分样本 |
| 正则与 mixed | lookaround、backreference、原始范围与逐阶段回退；设置验证、捕获规则、搜索使用同一引擎 | search/clipboard/settings 回归；整次搜索有 100ms 预算 |
| 键盘导航 | Ctrl-N/J/P/K、Shift 方向键、Cmd/Option 跳转、PageUp/PageDown、页脚循环；自定义快捷键按 KeyChord 优先级处理；键盘滚动不会被静止鼠标抢回选择 | `interaction_parity_test.go` |
| 搜索操作 | Ctrl-U 清空、Ctrl-H 按完整字素删除末字、Ctrl-W 删除末词；Escape 关闭窗口；无结果 Return 复制查询并清空搜索、保留窗口 | `interaction_parity_test.go`、原生窗口验证 |
| 编辑 | 别名允许为空，图片别名禁用；仅修改别名保留格式，修改正文移除旧 RTF/HTML；Return 保存、无改动不保存 | `interaction_parity_test.go` |
| 长文本 | 首个超过 10,000 字节的段落截断后续预览，不切 UTF-8 scalar；禁用截断文本的编辑，保留原内容 | `text_layout_test.go`、编辑器回归 |
| 生命周期与资源 | 退出清理保留置顶项；隐藏菜单栏图标跨重启生效；退出处理仅注册一次；淘汰/删除释放 bitmap 缓存，旧异步预览不能重新填回已删条目 | app/history/原生窗口回归 |
| 预览与设置 | 返回历史条目时重置自动预览抑制，取消过期计时及排队请求；设置不再叠画旧页面 | `auto_preview_test.go`、`settings_transition_test.go` |

### 验证边界与尚未同步的行为

- macOS 主快捷键已补应用层 NSEvent local monitor 与 opening/toggle/cycle 状态机；原生事件探针覆盖 keyDown/flagsChanged、窗口隔离和移除，真实全局组合键的完整人工往返仍待验收。非 macOS 保留普通开关；键位沿用 MyGo 的 Carbon ANSI 物理映射，尚未复刻 Swift Sauce 的完整布局转换。
- Linux 的剪贴板接口没有原生复制序号时仍用 fingerprint，无法识别完全相同内容的再次复制。
- 正则使用带超时的纯 Go regexp2；已覆盖常用前后查找和反向引用，不能把全部 .NET/ICU 专用语法视为等价。超过 64 个字素的非完整相等模糊模式安全返回不匹配，不复刻 Swift 位移溢出。
- iPhone/Handoff 的真实设备往返尚未验证；Universal Clipboard 已验证的是临时文件与数据表示协议。
- 剪贴板已先协商格式、预检敏感类型/忽略应用，再按需 Read(formats...)；必要的文本过滤表示仍会读取。无原生复制序号的平台依然受接口限制。
- JSON/blob 存储仍不具备 SwiftData 的数据库事务语义；已改为成功提交索引后回收，保护旧索引、保存失败的内存历史及异步捕获 lease。转换记录的会话映射跟随新条目，避免留下已删除对象引用。
- 仍未实现 App Intents、通知自定义音效；多选/粘贴栈在 Swift 快照中被常量禁用，权限检查也是空操作，二者不再误列为原版已开放能力。窗口边缘调整、可拖动内容/预览分栏及相对屏幕位置已经补齐。

### 本轮验证

- `CGO_ENABLED=0 go test ./... -count=1 -timeout=90s` 通过。
- `MACCY_NATIVE_CLIPBOARD_TEST=1 CGO_ENABLED=0 go test ./internal/clipboard -run TestNativeClipboardSequenceCountsIdenticalWrites -v -count=1 -timeout=30s` 通过；命名板连续相同写入序号为 `[0 1 2]`，系统板内容没有被读取或替换。
- Linux、Windows 纯 Go 编译通过；macOS `.app` 构建及 `codesign --verify --deep --strict` 通过；`go mod verify` 通过。
- 独立应用名、合成历史、禁用捕获的副屏窗口验证：无键置顶、RTF 别名/正文编辑、保存保护、长段落只读、Ctrl 导航、静止指针下的 End 滚动、图片/页脚往返三轮、带查询 Escape 关闭。
- 设置页连续切换 15 次，窗口锚点与最终尺寸保持一致，动画完成后没有继续改变尺寸。
- 顶部页签与原生窗口采用同一居中取整规则，修复奇偶宽度交替时的半点位移。深浅色、1×/2× 字体像素回归通过；原生窗口连续切换 20 次、采样 1,903 帧，六个页签的屏幕位置均保持一致。
- 原版 Fuse 1.4.0 Swift 二进制与 Go 实现的 270 个样本，匹配结果、评分、高亮范围一致。依赖来源及许可证保留于 `internal/search/LICENSE.fuse-swift`；一次性差分程序保存在本机临时目录。

## 2026-11-05 补充源码对齐

继续以本地 `f0b571e` 为对照，未更新或修改 reference 源码。本次实际修复：

| 触发条件 | 现在的行为 | 证据 |
| --- | --- | --- |
| Option / Option+Shift 点击历史行 | 传递点击时的修饰键，执行粘贴 / 去格式粘贴；多余修饰键不会误执行默认复制 | `tray_actions_test.go` 的真实行点击回归 |
| Option 点击菜单栏图标 | 暂停/恢复记录；加 Shift 只忽略下一次；普通点击固定在图标下方打开 | `tray_actions.go`、`AppDelegate.performStatusItemClick` 对照 |
| 图标选择与暂停状态 | 使用原版四份模板资源，重启恢复偏好；暂停或关闭所有保存类型时变淡并显示暂停提示 | `assets/maccy`（附 MIT 许可证）、图标区别回归 |
| 保持主快捷键修饰键连按 | 循环选择，全部松键确认；直接松键进入普通开关；编辑时忽略主快捷键 | 状态机回归、隔离 NSEvent 原生探针 |
| 编辑时失焦 | 保留编辑输入，不因普通失焦直接丢弃 | `OnBlur` 与 Swift `suppressPopupAutoClose` 对照 |
| 调整窗口/分栏 | 支持原生边缘调整，左右预览均可拖动；分栏可聚焦后用左右键调整，两栏各至少 200pt，尺寸保存 | `popup_geometry_test.go`、隔离原生窗口探针、副屏真实面板验证 |
| 更换屏幕/改变宽度后用上次位置 | 保存屏幕相对的顶部中心锚点；兼容原有绝对坐标；活动屏幕在 macOS 取 NSScreen.main | `popup_position_test.go`、`popup_screen_darwin.go` |
| 搜索高亮与自定义快捷键提示 | 四种高亮实际绘制，Color 为黄色背景黑字；工具条帮助跟随当前快捷键/清空绑定 | 深浅色像素回归、实际 hover 文案回归 |
| 剪贴板捕获与内容回收 | 敏感/禁用格式在读取前排除；索引提交后回收无引用内容，异步交接与保存失败保留数据 | clipboard provider 回归、model 提交/失败/lease 回归 |

原生副屏验证使用独立名称、合成历史及暂停捕获：左侧预览拖动到 530/320pt 后窗口保持 851pt，连续三轮开合保留锚点与栏宽；编辑取消回到保存的尺寸。没有读取或替换用户系统剪贴板。窗口截图与一次性自动化脚本保存在本机 `/tmp/maccy-alignment-*`。

实际更新发布仍需用户的签名密钥/发布仓库；App Intents 与自定义通知音效尚未移植。正则方言、超长模糊模式、Handoff 真机、Linux 相同内容复制计数、完整键盘布局转换继续保留上面的验证边界。查找高亮采用黄色，未绑定 macOS 自定义系统查找颜色；菜单栏的暂停提示使用 MyGo tooltip，未接 Swift 的独立 VoiceOver accessibilityLabel。


### 补充对齐验证结果

- `MACCY_NATIVE_GEOMETRY_TEST=1 MACCY_NATIVE_HOTKEY_TEST=1 CGO_ENABLED=0 go test ./... -count=1 -timeout=90s` 通过：包含隔离 AppKit 事件和原生窗口样式/尺寸探针。
- `go test -race ./internal/model -count=1 -timeout=45s` 通过；Linux、Windows 纯 Go 构建及 `go vet ./...` 通过。
- macOS 构建保存在 `build/alignment-20261105/darwin-arm64/Maccy.app`，ad hoc 签名验证通过。
- 原生窗口尺寸验证使用实际 NSWindow delegate 回调，副屏验证覆盖实际面板和分隔线输入；真实鼠标拖窗口边缘和完整全局组合键往返仍保留人工验收边界。
