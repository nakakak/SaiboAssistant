package ui

// GuideZh 内置使用说明（连接设置窗口也可打开本页）。
const GuideZh = `赛搏小小助手 — 使用说明

━━━━━━━━━━━━━━━━
〇、首次使用需要哪些信息（总览）
━━━━━━━━━━━━━━━━
【云端 / 设备 — 先具备】
• 商家云已运行，且 bridge_enabled 已开启（设备能进 OpenClaw / 听写页）。
• 喵伴已在当前商家云「设备管理」中注册并绑定智能体。

【助手 config.yaml / 向导 — 必填】
• server_url：ws(s)://主机:端口/bridge/connector（局域网默认 ws://192.168.50.176:8084/bridge/connector）。
• 设备标识二选一：6 位 pair_code（推荐）或 device_mac（管理台复制）。

【按功能额外需要】
• OpenClaw：本机 openclaw gateway（默认 18789）+ gateway.auth.token + openclaw.mode=gateway_ws。
• 听写：channels.telnet=true，dictation.inject=true；macOS 辅助功能允许本程序。

【配对码从哪看】
• 喵伴 OpenClaw 对话页 或 听写转发页，仅 Connector 离线时显示「配对码 XXXXXX」。
• 赛搏助手已连接时屏上会隐藏码；需查看时先退出助手再进设备页。

━━━━━━━━━━━━━━━━
一、功能通道与首次向导
━━━━━━━━━━━━━━━━
• 首次将 first_run 设为 true 时：欢迎 → 勾选 OpenClaw / 听写 → 填商家云与配对 → 权限同意 → 托盘常驻。
• 日常在托盘「通道设置」修改；至少保留一个通道开启。
• 关闭 Telnet 后不处理听写；关闭 OpenClaw 后云端任务会收到「本机未开启」类回执。

━━━━━━━━━━━━━━━━
二、云端连接（必填）
━━━━━━━━━━━━━━━━
1. server_url 须含 /bridge/connector。
2. 只填 6 位验证码时，程序向同主机 HTTP API 解析 device_mac 并写回配置。
3. 验证码与设备屏一致；默认长期不变。也可直接填 device_mac。

━━━━━━━━━━━━━━━━
三、OpenClaw（任务派发）
━━━━━━━━━━━━━━━━
1. 本机运行 openclaw gateway（默认 18789）。
2. gateway.auth.token → 连接设置「Gateway Token」。
3. openclaw.mode=gateway_ws；地址 http://127.0.0.1:18789（Gateway 在别机则填该机）。
4. mode=echo 时不连真实 OpenClaw，仅链路自测。

━━━━━━━━━━━━━━━━
四、听写转发
━━━━━━━━━━━━━━━━
1. 设备听写转发页 → 开始传输；Mac 先点好输入框。
2. dictation.subtitle=false：仅注入，无悬浮窗（推荐）。
3. macOS：辅助功能允许本程序；Windows/Linux 见 README。

━━━━━━━━━━━━━━━━
五、托盘与命令行
━━━━━━━━━━━━━━━━
• 连接设置 / 重连云端 / 通道设置 / 退出。
• ./SaiboAssistant --pair 123456 --headless  无界面一次性配对。
`
