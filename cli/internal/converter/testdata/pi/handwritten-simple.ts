import type { ExtensionContext } from "@badlogic/pi";

export function activate(ctx: ExtensionContext): void {
  ctx.hooks.on("tool_call", (event) => {
    console.log("tool called:", event.tool);
  });

  ctx.hooks.on("session_start", () => {
    console.log("session started");
  });
}
