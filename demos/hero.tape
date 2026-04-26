# Hero demo: TUI library browse + search + cross-provider install
#
# Shows: launch → library browse → search → install wizard → success
#
# Navigation reference (current TUI):
#   1 / 2 / 3  — switch top-level groups (Collections, Content, Config)
#   Tab        — cycle sub-tabs within a group
#   ↑/↓        — navigate list items
#   /          — open search
#   i          — open install wizard on selected item
#
# Demo installs "code-review" skill (shared library) to Gemini CLI.
# Gemini CLI is the 2nd provider in the wizard list (Down once from Claude Code).
#
# Review step requires Tab (focus buttons) + Right (cursor to Install) + Enter.
#
# Record: cd /path/to/syllago/demos && vhs hero.tape

Output "hero.gif"

Set Shell "bash"
Set FontSize 15
Set Width 1200
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

# Launch syllago TUI from the demo project
Type "cd ~/demo-project && syllago"
Enter
Sleep 3s

# Pause on Collections > Library — let the viewer see the layout
Sleep 2s

# Browse a few library items
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

# Search from Library directly — type "/" then filter
Type "/"
Sleep 400ms
Type "code-review"
Sleep 800ms

# Confirm search (exits input, keeps filter — code-review is selected)
Enter
Sleep 600ms

# Open install wizard
Type "i"
Sleep 2s

# Step 1: Provider — Down once selects Gemini CLI (2nd after Claude Code)
Down
Sleep 400ms
Enter
Sleep 2s

# Step 2: Location — accept default install path
Enter
Sleep 1.5s

# Step 3: Method — accept default (symlink)
Enter
Sleep 1.5s

# Step 4: Review — Tab to buttons zone, Right to Install button, Enter to confirm
Tab
Sleep 400ms
Right
Sleep 300ms
Enter
Sleep 3s

# Success toast visible — pause to show it
Sleep 2s

# Quit
Type "q"
Sleep 1s
