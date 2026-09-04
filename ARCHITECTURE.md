# 轻量版架构

## 运行链路

```text
Windows 应用
    │
TSF DLL (x86 / x64)
    ├─ 按键与 Composition
    ├─ 候选窗
    └─ AI 提示窗
    │  Named Pipe（有界异步等待）
Launcher / Broker
    │
Go IME Core
    ├─ Rime + 白霜拼音 + userdb
    └─ Local AI Completion
           │
       127.0.0.1 OpenAI-compatible service
```

## 边界

- TSF 层只做输入、窗口和管道通信，不访问模型、网络或词库文件。
- Rime 负责候选、词频学习和用户库合并，不在 Go 层重复实现 Ranker。
- 产品配置只暴露 `rime_frost` 全拼和 `rime_frost_double_pinyin_flypy`；沿用原方案 ID，升级后继续使用既有 userdb。
- AI 是可取消的可选增强；请求序号、超时和上下文检查阻止过期结果上屏。
- Launcher 不监听系统剪贴板，不维护 WebDAV 或云端同步状态。
- Windows 主工程不再携带 Android bridge、Android 原生库或移动端构建依赖。
- 只有一套产品视觉系统；旧外观配置只用于迁移输入行为，不能覆盖新样式。

## AI 生命周期

- 智能：450ms 防抖预计算，首次请求懒启动，10/30 分钟或永不空闲退出。
- 常驻：输入法初始化后预热，不设空闲计时器。
- 手动：不预计算，只在 `F8` / `Shift+F8` 时启动或请求。

运行时只终止它自己启动的 llama-server，不会关闭用户已有的服务进程。

## 验证

```powershell
cd src/moqi-ime
go test ./input_methods/rime -run '^(TestProductSettings|TestParseLocalAIPID|TestLightweightMenu|TestNewInitialState|TestGhostCompletion|TestAIConfigExplicitEmptyActions)'
go build -trimpath -ldflags '-s -w' .
```

Windows 前端需在 MSVC Developer Command Prompt 中生成 x86/x64 TSF DLL，并在 x86 配置下生成 Launcher。如果项目绝对路径含非 ASCII 字符，旧版 `protoc.exe` 需通过纯 ASCII 目录联接运行。

构建包会保留白霜基础/细胞词库、`essay.txt` 与 `zh-moqi.gram`，只排除产品方案未引用的翻译、Emoji、OpenCC、五笔/T9 和开发资料。体积是结果指标，不以牺牲候选质量为代价。

安装暂存使用 `scripts/install.ps1 -StageOnly`。该模式会验证并组装 x86 Launcher/SetupHelper/TSF、x64 TSF 和后端公共运行时，但不会写注册表或安装系统输入法；Inno Setup 仅是发布正式安装器时的可选最后一步。
