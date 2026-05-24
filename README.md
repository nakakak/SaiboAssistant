# SaiboAssistant

**赛搏小小助手** v0.2.14 — 本仓库提供 **Release 单文件安装包** 下载（`SaiboAssistant-*`），不含源码。

## 快速开始

1. 打开 **[v0.2.14 Release](https://github.com/nakakak/SaiboAssistant/releases/tag/v0.2.14)**，下载与你系统对应的 `SaiboAssistant-*` 单文件。
2. 阅读 **[下载与配置指南.md](./下载与配置指南.md)**，完成首次向导配置。

## 文档

| 文档 | 内容 |
|------|------|
| [下载与配置指南.md](./下载与配置指南.md) | v0.2.14 单文件下载、安装、向导配置、MAC/Token、日常使用（含 [配置截图](./docs/images/)） |

## v0.2.14 下载直链

- [Windows x64](https://github.com/nakakak/SaiboAssistant/releases/download/v0.2.14/SaiboAssistant-Windows-x64.exe)
- [macOS arm64](https://github.com/nakakak/SaiboAssistant/releases/download/v0.2.14/SaiboAssistant-macOS-arm64)
- [macOS x64](https://github.com/nakakak/SaiboAssistant/releases/download/v0.2.14/SaiboAssistant-macOS-x64)
- [Linux x64](https://github.com/nakakak/SaiboAssistant/releases/download/v0.2.14/SaiboAssistant-Linux-x64)
- [Linux arm64](https://github.com/nakakak/SaiboAssistant/releases/download/v0.2.14/SaiboAssistant-Linux-arm64)

## v0.2.14 相对 v0.2.13 的改进

- 修复切换商家云后，运行状态窗口**仍显示旧 server_url** 的问题（保存/重连后会刷新）。

## 设备配对说明（v0.2.14）

- ✅ **仅支持设备 MAC 地址** 连接商家云。
- ❌ **6 位验证码 / 连接码尚不能使用**（界面可能仍显示，请勿选择）；该能力为 **后期实现**，后续版本会更新文档与 Release 说明。

## 商家云地址（与 xiaozhi.cyberai.top 对齐）

| 用途 | 地址 |
|------|------|
| 商家云登录 | https://xiaozhi.cyberai.top |
| **赛搏小小助手 server_url** | **`wss://xiaozhi.cyberai.top/bridge/connector`** |
| 设备 OTA（固件） | `https://xiaozhi.cyberai.top/api/ota/` |

局域网测试见 [下载与配置指南.md](./下载与配置指南.md) 中的环境对照表。

## v0.2.13 及更早版本要点

- 首次向导不预填商家云地址；修复配置完成后界面崩溃；听写默认仅 inject；托盘常驻。
