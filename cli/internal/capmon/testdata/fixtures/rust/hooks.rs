// hooks.rs — provider hook event definitions (fixture for capmon tests)

pub enum HookEvent {
    PreToolUse,
    PostToolUse,
    Stop,
    Notification,
}

pub enum ToolName {
    BashTool,
    ReadTool,
    WriteTool,
}

pub const HOOK_VERSION: &str = "2024.1";
pub const MAX_HOOKS: u32 = 64;

pub struct HookConfig {
    pub event: HookEvent,
    pub blocking: bool,
    pub command: Option<String>,
}

pub trait HookHandler {
    fn handle(&self, event: HookEvent) -> HookResult;
}

pub struct HookResult {
    pub decision: String,
    pub reason: Option<String>,
}
