import { spawn } from "node:child_process";
import { readFileSync, existsSync, writeFileSync, mkdirSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, join } from "node:path";
import WebSocket from "ws";

const SCHEMA = 1;
export const DEFAULT_SERVER =
  process.env.MIAOBAN_BRIDGE_SERVER_URL?.trim() ||
  "ws://192.168.50.176:8084/bridge/connector";

export function defaultConfigPath() {
  return join(homedir(), ".miaoban-bridge", "config.yaml");
}

export function parseArgs(argv) {
  const out = { pair: "", server: DEFAULT_SERVER, config: "", help: false };
  const args = argv.slice(2);
  let i = 0;
  if (args[0] === "miaoban-bridge" || args[0] === "bridge") {
    i = 1;
  }
  for (; i < args.length; i++) {
    const a = args[i];
    if (a === "--pair" || a === "-p") out.pair = (args[++i] || "").trim();
    else if (a === "--server") out.server = (args[++i] || "").trim();
    else if (a === "--config") out.config = (args[++i] || "").trim();
    else if (a === "--help" || a === "-h") out.help = true;
    else if (/^\d{6}$/.test(a)) out.pair = a;
  }
  return out;
}

export function printHelp() {
  process.stdout.write(
    "miaoban-bridge — 喵伴设备 ↔ 商家云 ↔ 本机 OpenClaw（小龙虾）\n\n" +
      "用法:\n" +
      "  miaoban-bridge --pair <喵伴 OpenClaw 页屏幕上的 6 位配对码>\n\n" +
      "前提:\n" +
      "  1. 电脑已安装 OpenClaw（官方或第三方发行版均可）\n" +
      "  2. 已运行: openclaw gateway\n" +
      "  3. 喵伴已在商家云后台完成绑定\n\n" +
      "环境变量:\n" +
      "  MIAOBAN_BRIDGE_SERVER_URL  商家云桥接地址（默认公网云）\n" +
      "  MIAOBAN_OPENCLAW_AGENT     OpenClaw agent id（默认 main）\n\n" +
      "配置（可选）:\n" +
      `  ${defaultConfigPath()}\n`,
  );
}

function deriveApiBase(wsUrl) {
  const u = new URL(wsUrl);
  const scheme = u.protocol === "wss:" ? "https:" : "http:";
  return `${scheme}//${u.host}`;
}

async function resolvePairCode(serverUrl, code) {
  const base = deriveApiBase(serverUrl);
  const res = await fetch(`${base}/api/public/connector-code-resolve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(`配对码无效或已过期: HTTP ${res.status}`);
  }
  const mac = body?.data?.device_mac?.trim();
  if (!mac) throw new Error("配对码解析失败: 无 device_mac");
  return mac;
}

function loadConfig(path) {
  if (!path || !existsSync(path)) return {};
  try {
    const raw = readFileSync(path, "utf8");
    const out = {};
    for (const line of raw.split("\n")) {
      const m = line.match(/^\s*([a-z_]+)\s*:\s*"?([^"#]+)"?\s*$/);
      if (m) out[m[1]] = m[2].trim();
    }
    return out;
  } catch {
    return {};
  }
}

function saveDeviceMac(configPath, mac) {
  if (!configPath) return;
  try {
    mkdirSync(dirname(configPath), { recursive: true });
    let existing = existsSync(configPath) ? readFileSync(configPath, "utf8") : "";
    if (!existing.trim()) {
      existing = `# miaoban-bridge 配置\nserver_url: "${DEFAULT_SERVER}"\n`;
    }
    if (existing.includes("device_mac:")) {
      writeFileSync(
        configPath,
        existing.replace(/device_mac:\s*.*/, `device_mac: "${mac}"`),
        "utf8",
      );
    } else {
      writeFileSync(configPath, `${existing}device_mac: "${mac}"\n`, "utf8");
    }
  } catch {
    /* ignore */
  }
}

function buildConnectorURL(serverUrl, deviceMac) {
  const u = new URL(serverUrl);
  u.searchParams.set("device_mac", deviceMac);
  return u.toString();
}

function runOpenClawAgent(message) {
  const agentId =
    process.env.MIAOBAN_OPENCLAW_AGENT?.trim() ||
    process.env.OPENCLAW_AGENT?.trim() ||
    "main";
  const args = ["agent", "--agent", agentId, "--message", message, "--json"];
  return new Promise((resolve, reject) => {
    const child = spawn("openclaw", args, {
      shell: process.platform === "win32",
      env: process.env,
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (d) => {
      stdout += d;
    });
    child.stderr.on("data", (d) => {
      stderr += d;
    });
    child.on("error", (err) => {
      if (err?.code === "ENOENT") {
        reject(
          new Error(
            "未找到 openclaw 命令。请先安装 OpenClaw（小龙虾）并确保 openclaw 在 PATH 中。",
          ),
        );
        return;
      }
      reject(err);
    });
    child.on("close", (code) => {
      if (code !== 0) {
        reject(new Error(stderr.trim() || stdout.trim() || `openclaw agent 退出码 ${code}`));
        return;
      }
      const text = stdout.trim();
      try {
        const j = JSON.parse(text);
        const reply =
          j?.result?.payloads?.[0]?.text ||
          j?.text ||
          j?.message ||
          j?.response ||
          text;
        resolve(String(reply || text).trim() || message);
      } catch {
        resolve(text || message);
      }
    });
  });
}

function sendTaskResult(ws, taskId, display, speak) {
  ws.send(
    JSON.stringify({
      type: "task.result",
      schema_version: SCHEMA,
      task_id: taskId,
      result: { display_text: display, speak_text: speak },
      ts_ms: Date.now(),
    }),
  );
}

function sendTaskFailed(ws, taskId, message) {
  ws.send(
    JSON.stringify({
      type: "task.failed",
      schema_version: SCHEMA,
      task_id: taskId,
      error: { code: "TOOL_ERROR", message: message || "任务执行失败" },
      ts_ms: Date.now(),
    }),
  );
}

async function handleDispatch(ws, msg) {
  const taskId = msg.task_id;
  const text = msg?.input?.text || "";
  if (!taskId) return;
  console.log(`[miaoban-bridge] 收到任务 task_id=${taskId}`);
  try {
    const speak = await runOpenClawAgent(text);
    sendTaskResult(ws, taskId, speak, speak);
    console.log(`[miaoban-bridge] 已回传结果 task_id=${taskId}`);
  } catch (err) {
    console.error(`[miaoban-bridge] 任务失败 task_id=${taskId}`, err?.message || err);
    sendTaskFailed(ws, taskId, err?.message || String(err));
  }
}

export async function runBridge(argv = process.argv) {
  const args = parseArgs(argv);
  if (args.help) {
    printHelp();
    return;
  }

  const configPath = args.config || defaultConfigPath();
  const fileCfg = loadConfig(configPath);
  const serverUrl = args.server || fileCfg.server_url || DEFAULT_SERVER;
  let deviceMac = (fileCfg.device_mac || "").trim();
  const pair = (args.pair || fileCfg.pair_code || "").replace(/\D/g, "");

  if (!deviceMac && pair.length === 6) {
    deviceMac = await resolvePairCode(serverUrl, pair);
    saveDeviceMac(configPath, deviceMac);
    console.log(`[miaoban-bridge] 配对成功 device_mac=${deviceMac}`);
  }

  if (!deviceMac) {
    console.error("用法: miaoban-bridge --pair <喵伴屏幕6位配对码>\n帮助: miaoban-bridge --help");
    process.exit(1);
  }

  const url = buildConnectorURL(serverUrl, deviceMac);
  console.log("[miaoban-bridge] 正在连接商家云…");

  const connect = () => {
    const ws = new WebSocket(url);
    ws.on("open", () => console.log("[miaoban-bridge] 已连接商家云"));
    ws.on("message", (data) => {
      let msg;
      try {
        msg = JSON.parse(String(data));
      } catch {
        return;
      }
      if (msg?.type === "bridge.connector_ready") {
        console.log("[miaoban-bridge] 就绪 — 请在喵伴 OpenClaw 页点「开始对话」");
        return;
      }
      if (msg?.type === "task.dispatch") void handleDispatch(ws, msg);
    });
    ws.on("close", () => {
      console.log("[miaoban-bridge] 连接断开，2 秒后重连…");
      setTimeout(connect, 2000);
    });
    ws.on("error", (err) => console.error("[miaoban-bridge] 连接错误:", err.message));
  };
  connect();
}
