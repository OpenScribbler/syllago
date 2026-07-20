import { execSync } from "child_process";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.on("tool_call", (event, ctx) => {
    if (event.toolName !== "bash") return;
    try {
      execSync("echo check", { stdio: "pipe", timeout: 5000 });
    } catch (err: any) {
      if (err.status === 2 || err.killed === true || err.status === null) {
        return { block: true, reason: err.stderr?.toString() || "hook failed" };
      }
    }
  });

  pi.on("session_start", (event, ctx) => {
    execSync("echo init", { stdio: "pipe" });
  });
}
