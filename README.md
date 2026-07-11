# SaiboAssistant（赛搏小小助手）

喵伴 ↔ 商家云 ↔ 本机 **OpenClaw / 听写** 的连接器。

| 组件 | 说明 |
|------|------|
| **GUI**（`SaiboAssistant` / `cmd/openclaw-connector`） | 托盘程序，向导支持 **6 位配对码** 或 MAC |
| **CLI**（`miaoban-bridge` / `lib/bridge.mjs`） | 终端轻量桥接，`--pair` 配对 |

**连接使用指南：** [下载与配置指南.md](./下载与配置指南.md)（下载 → 配对 → 连上，五步完成）

---

## 版本与下载

| 版本 | GitHub | 6 位连接码 | 获取方式 |
|------|--------|------------|----------|
| **v0.2.19**（当前 Latest，推荐） | [Release](https://github.com/nakakak/SaiboAssistant/releases/tag/v0.2.19) | ✅ | 听写聊天不换行；勾选听写通道自动开启 |
| **v0.2.18** | [Release](https://github.com/nakakak/SaiboAssistant/releases/tag/v0.2.18) | ✅ | 修复 Windows 听写注入 |
| **v0.2.17** | [Release](https://github.com/nakakak/SaiboAssistant/releases/tag/v0.2.17) | ✅ | 内置 `wss://xiaozhi.cyberai.top/bridge/connector` |
| **v0.2.16** | [Release](https://github.com/nakakak/SaiboAssistant/releases/tag/v0.2.16) | ✅ | 内置 `ws://192.168.50.52:8084/bridge/connector` |
| **v0.2.15** | [Release](https://github.com/nakakak/SaiboAssistant/releases/tag/v0.2.15) | ✅ | 内置 `192.168.50.176` |
| **v0.2.14** | [Release](https://github.com/nakakak/SaiboAssistant/releases/tag/v0.2.14) | ❌ 仅 MAC | 历史版本 |

### 各操作系统 Release 文件（v0.2.19）

| 系统 | CPU | 下载文件 |
|------|-----|----------|
| Windows | x64 | `SaiboAssistant-Windows-x64.exe` |
| macOS | Apple Silicon (M 系列) | `SaiboAssistant-macOS-arm64` |
| macOS | Intel | `SaiboAssistant-macOS-x64` |
| Linux | x86_64 | `SaiboAssistant-Linux-x64` |
| Linux | arm64 | `SaiboAssistant-Linux-arm64` |

直链示例（Apple Silicon）：

```bash
curl -fL -o SaiboAssistant-macOS-arm64 \
  https://github.com/nakakak/SaiboAssistant/releases/download/v0.2.19/SaiboAssistant-macOS-arm64
chmod +x SaiboAssistant-macOS-arm64 && ./SaiboAssistant-macOS-arm64
```

**需要连接码？** 下载 **v0.2.15** 即可；也可源码构建：

```bash
cd SaiboAssistant
go build -trimpath -ldflags="-s -w" -o SaiboAssistant ./cmd/openclaw-connector
./SaiboAssistant
```

---

## 快速开始（GUI · v0.2.15+）

1. 商家云、设备绑定、`openclaw gateway`（若用 OpenClaw）已就绪。
2. 喵伴 **OpenClaw / 听写转发** 页查看 **6 位配对码**（助手未连接时显示）。
3. 运行 SaiboAssistant → 首次向导：勾选功能 → 填 **连接码** → Gateway Token → 完成。
4. 详见 [下载与配置指南.md](./下载与配置指南.md)。

```bash
./SaiboAssistant --pair 123456 --headless   # 无界面一次性配对
```

## 快速开始（CLI）

```bash
npm install && npm install -g .
openclaw gateway
export MIAOBAN_BRIDGE_SERVER_URL="wss://xiaozhi.cyberai.top/bridge/connector"
miaoban-bridge --pair 123456
```

---

## v0.2.19 变更

- **听写聊天模式**：多句合并用空格连接，对话框不再自动换行
- **连接设置简化**：勾选「听写转发」通道即自动开启 dictation.stt 与 inject，无需单独勾选

## v0.2.18 变更

- **修复 Windows 听写注入**：移除误用的 macOS target 检查，识别文字可正常写入当前焦点输入框（使用前请先点一下目标输入框）

## v0.2.17 变更

- 内置商家云地址改为 `wss://xiaozhi.cyberai.top/bridge/connector`（公网云）；局域网仍可用环境变量或连接设置覆盖

## v0.2.16 变更

- 内置商家云地址改为 `ws://192.168.50.52:8084/bridge/connector`

## v0.2.15 新特性

- 向导 / 连接设置支持 **6 位连接码**（自动解析 `device_mac`）
- 商家云默认 `wss://xiaozhi.cyberai.top/bridge/connector`（可用环境变量覆盖）
- `--pair` 命令行配对
- 听写转发：流式注入、清除按钮、配对码在听写页显示（需设备固件配合）

---

## 文档

| 文档 | 内容 |
|------|------|
| [下载与配置指南.md](./下载与配置指南.md) | **全平台 Release 下载**、安装、向导、配对码、MAC、托盘 |
| [BRIDGE_CLI.md](./BRIDGE_CLI.md) | npm CLI（miaoban-bridge） |

---

## 打包 Release

```bash
./scripts/package-release.sh v0.2.15        # 本机单平台
./scripts/package-release.sh v0.2.15 --all  # 五平台 + checksums
```

产物在 `dist/`：`SaiboAssistant-<OS>-<arch>` 及 `openclaw-connector_<ver>_<os>_<arch>.tar.gz/.zip`。

推 tag `v*` 后 GitHub Actions 自动发布（见 `.github/workflows/release.yml`）。

---

## 商家云地址

| 用途 | 地址 |
|------|------|
| 管理台登录（公网） | https://xiaozhi.cyberai.top |
| 连接器 `server_url`（内置默认） | `wss://xiaozhi.cyberai.top/bridge/connector` |
| 局域网测试（可选） | `ws://192.168.50.52:8084/bridge/connector` |
| 配对码解析 API | `POST /api/public/connector-code-resolve` |
