# Import demo: discover and add hooks + MCP from Claude Code
#
# Shows: discovery (both scopes) -> add hooks -> add MCP -> scope metadata
#
# Prerequisites:
#   1. cd ~/demo-project (has .claude/settings.json with hooks + MCP)
#   2. Clean previous imports if needed

Output "import-hooks-mcp.gif"

Set Shell "bash"
Set FontSize 15
Set Width 1000
Set Height 550
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

# Step 1: Discover what's available
Type "syllago add --from claude-code 2>&1 | head -25"
Enter
Sleep 5s

# Step 2: Add MCP configs (shows both scopes)
Type "syllago add mcp --from claude-code --dry-run"
Enter
Sleep 5s

# Step 3: Add hooks (shows both scopes)
Type "syllago add hooks --from claude-code --scope=project --dry-run"
Enter
Sleep 5s

# Step 4: Show what's in the library now
Type "syllago list mcp 2>&1 | head -15"
Enter
Sleep 4s

Sleep 2s
