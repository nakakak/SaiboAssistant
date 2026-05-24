# 配置截图目录

把赛搏小小助手的配置界面截图放在本目录，供 [下载与配置指南.md](../../下载与配置指南.md) 引用。

## 推荐文件名（v0.2.14 图文包）

**请把 PNG/JPG 放在本目录** `SaiboAssistant/docs/images/`（不要放在仓库根目录）。

### 必传（当前 5 张）

| 保存为 | 你的截图内容 |
|--------|----------------|
| `03-first-run-welcome.png` | 首次向导 — 勾选 OpenClaw / 听写 等功能 |
| `04-wizard-server-mac-openclaw.png` | 首次向导 — 商家云地址 + 设备 MAC + Gateway Token |
| `10-wizard-consent.png` | 首次向导 — 阅读说明与资源使用同意勾选 |
| `06-status-connected.png` | 运行状态 — **已连接商家云** |
| `09-admin-device-mac.png` | 赛搏管理后台 — 设备管理中查看 **设备 MAC** |

### 可选（有则再放）

| 文件名 | 内容 |
|--------|------|
| `01-download-release.png` | GitHub Release 下载页 |
| `02-mac-gatekeeper.png` | Mac「无法打开」或右键「仍要打开」 |
| `07-settings-server-url.png` | 连接设置 — 修改 server_url |
| `08-tray-menu.png` | 菜单栏托盘菜单 |

格式：**PNG** 或 **JPG**，宽度建议 800–1200px，便于在 GitHub 上阅读。

## 如何添加

1. 截图保存到本目录，使用上表文件名（可增删）。
2. 在 `下载与配置指南.md` 中已预留 `![说明](./docs/images/xxx.png)`，把 `xxx` 改成你的文件名即可。
3. 提交并推送：

```bash
cd /path/to/SaiboAssistant
git add docs/images/ 下载与配置指南.md
git commit -m "docs: add SaiboAssistant setup screenshots"
git push
```

GitHub 上会直接显示图片；Release 说明里也可贴同路径图片（需已在仓库中）。

## 截图时注意

- 可打码：Token、内网 IP、完整 MAC（若不想公开）。
- 文档说明：**当前仅支持 MAC 配对**；连接码截图勿作为 v0.2.14 操作指引。
- 公网云示例地址：`wss://xiaozhi.cyberai.top/bridge/connector`
- 不要与「管理台 WebSocket `.../ws`」混淆，助手必须是 `.../bridge/connector`
