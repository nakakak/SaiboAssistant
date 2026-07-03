# miaoban-bridge CLI（本仓库 npm 部分）

无需 GUI 时，用终端连接器即可。

## 安装

```bash
cd SaiboAssistant
npm install
npm install -g .
```

## 使用

```bash
openclaw gateway          # 终端 1，保持运行
miaoban-bridge --pair 123456   # 终端 2，码见喵伴 OpenClaw 页
```

配置写入 `~/.miaoban-bridge/config.yaml`。

## 开发

```bash
node run.mjs --pair 123456
```

## 环境变量

| 变量 | 说明 |
|------|------|
| `MIAOBAN_BRIDGE_SERVER_URL` | 商家云 WebSocket（默认公网云） |
| `MIAOBAN_OPENCLAW_AGENT` | OpenClaw agent id（默认 `main`） |
