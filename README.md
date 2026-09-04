# U 输入法

面向个人长期使用的 Windows 10/11 智能拼音输入法。核心路径是 TSF + Rime/白霜拼音，本地 AI 只作为可选续写增强；AI 未启动、超时或崩溃都不影响普通输入。

## 产品边界

保留：

- Windows TSF 中文输入、中英文/全半角/标点切换；
- Rime 白霜拼音、用户词频学习和自定义短语；
- 本地词库快照导入、导出与合并；
- 本地 OpenAI 兼容模型续写，支持智能、常驻、手动三种运行模式；
- 简化设置、日志和数据目录诊断。

不包含换肤、字号/颜色微调、翻译、语音、云剪贴板、WebDAV、在线方案下载、通用云端 AI 或插件市场。

## 交互

右键菜单只保留五个一级入口：中英文切换、输入方案、本地 AI、个人词库、设置与诊断。候选窗默认横排 7 个候选，使用 `mist-shore + functional` 语义色、Noto Sans SC 和稀疏的柿橙焦点色。

AI 快捷键：

- `F8`：请求或接受短续写；
- `Shift+F8`：请求长续写；
- `F9`：切换下一条建议；
- `Esc` 或继续输入：取消当前建议。

## 本地 AI

1. 安装 llama.cpp：`winget install --id ggml.llamacpp --exact`。
2. 将 `Qwen3.5-4B-Q4_K_M.gguf` 放到 `%LOCALAPPDATA%\MoqiAI\models\`。
3. 首次打开 AI 配置时，程序会创建 `%APPDATA%\Moqi\ai_config.json`。
4. 智能模式在首次中文续写请求时懒启动服务，空闲后只会关闭由本程序启动的进程。

## 目录

- `src/moqi-im-windows`：TSF DLL、Launcher/Broker 和候选窗。
- `src/moqi-ime`：Go 后端、Rime 桥接和 AI 续写。
- `src/moqi-ime/product-data`：产品自有的精简 Rime 配置；保留原方案 ID 以继承 userdb。
- `local-ai`：本地模型服务的懒启动脚本。
- `ARCHITECTURE.md`：轻量化边界、运行链路和故障隔离。
- [`src/moqi-ime/docs/ai-completion-acceptance-plan.md`](src/moqi-ime/docs/ai-completion-acceptance-plan.md)：AI 续写接受率研究、分阶段工程方案和测试方法。
- [`src/moqi-ime/docs/pinyin-reranker-training.md`](src/moqi-ime/docs/pinyin-reranker-training.md)：提交前拼音候选重排的训练与评估记录。
- `DESIGN.md` / `UI-GUIDANCE.md`：视觉系统与产品语义准则。

## 构建

运行 `src/moqi-ime/scripts/build.ps1` 可生成后端运行包。脚本固定白霜依赖版本；本地缺失时才下载，日常增量构建不执行 `go mod tidy`，也不强制安装版本资源工具。

本地验证产物位于 `build/artifacts/`，完整后端运行包位于 `build/package/moqi-ime/`（均由 Git 忽略）。当前产品数据只列出白霜全拼和小鹤双拼，保留完整基础/细胞词库、`essay.txt`、`zh-moqi.gram` 和 Rime userdb 学习。

前端 x86/x64 构建完成后，可先生成不会改动系统的安装暂存目录：

```powershell
src/moqi-im-windows/scripts/install.ps1 `
  -Win32BuildDir <x86-build> `
  -X64BuildDir <x64-build> `
  -MoqiImeSource build/package/moqi-ime `
  -StageDir build/installer-stage `
  -StageOnly
```

`-StageOnly` 只组装和检查文件，不注册 TSF、不启动 Launcher，也不需要 Inno Setup。需要生成正式安装程序时，再安装 Inno Setup 6 并去掉该参数。
