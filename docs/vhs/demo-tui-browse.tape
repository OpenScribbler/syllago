# Demo 1: TUI Browsing
# Shows launching the TUI, navigating groups and sub-tabs, and browsing content.
#
# Navigation reference (current TUI):
#   1 / 2 / 3  — switch top-level groups (Collections, Content, Config)
#   Tab        — cycle sub-tabs within a group
#   ↑/↓        — navigate list items
#   /          — search
#
# Collections sub-tabs: Library → Registries → Loadouts
# Content sub-tabs:     Skills → Agents → MCP → Rules → Hooks → Commands
#
# Record: cd ~/demo-project && vhs docs/vhs/demo-tui-browse.tape

Output docs/vhs/demo-tui-browse.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 800
Set Padding 20
Set TypingSpeed 60ms

# Launch syllago TUI — lands on Collections > Library
Type "syllago"
Enter
Sleep 2s

# Browse library items
Down
Sleep 500ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

# Switch to Content group — starts on Skills
Type "2"
Sleep 1s

# Navigate skills
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

# Tab to Agents
Tab
Sleep 800ms

# Tab to Rules
Tab
Sleep 400ms
Tab
Sleep 400ms

# Browse a few rules
Down
Sleep 400ms
Down
Sleep 400ms

# Back to Collections
Type "1"
Sleep 1s

# Tab to Registries, then Loadouts
Tab
Sleep 600ms
Tab
Sleep 800ms

# Pause on Loadouts gallery
Sleep 2s

# Quit
Type "q"
Sleep 1s
