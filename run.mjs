#!/usr/bin/env node
/**
 * monorepo 内 CLI：与全局 miaoban-bridge 行为一致。
 * 用户也可在本目录 npm install && npm install -g .
 */
import { runBridge, printHelp } from "./lib/bridge.mjs";

export function printBridgeHelp() {
  printHelp();
}

export async function runMiaobanBridge(argv = process.argv) {
  return runBridge(argv);
}

const isDirectRun =
  process.argv[1] &&
  (process.argv[1].endsWith("run.mjs") ||
    process.argv[1].includes("miaoban-bridge"));
if (isDirectRun) {
  runBridge().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
