// hooks.ts — provider hook event type definitions (fixture for capmon tests)

export enum HookEvent {
  PreToolUse = "PreToolUse",
  PostToolUse = "PostToolUse",
  Stop = "Stop",
  Notification = "Notification",
}

export enum ToolName {
  BashTool = "BashTool",
  ReadTool = "ReadTool",
  WriteTool = "WriteTool",
}

export const HOOK_VERSION = "2024.1";

export interface HookConfig {
  event: HookEvent;
  blocking: boolean;
  command?: string;
}

export type HookResult = {
  decision: "allow" | "deny";
  reason?: string;
};
