# IME

这是当前墨齐 Windows 输入法的开发快照，包含 TSF 输入法源码、AI 灰色续写功能、安装/更新脚本和本地 AI 启动配置。

## 目录

- `src/moqi-im-windows`：墨齐输入法源码及第三方依赖。
- `scripts`：本地构建、注册和部署脚本。
- `local-ai`：Qwen3.5-4B 本地推理启动脚本与输入法配置模板。
- `DESIGN.md`：项目架构与设计记录。

## 本地 AI

1. 安装 llama.cpp：`winget install --id ggml.llamacpp --exact`
2. 把 `Qwen3.5-4B-Q4_K_M.gguf` 放入 `local-ai/models/`。
3. 将 `local-ai/ai_config.example.json` 复制为 `%APPDATA%/Moqi/ai_config.json`。
4. 运行 `local-ai/启动本地AI.cmd`。

大模型、日志、构建产物和本机工具不纳入 Git。

