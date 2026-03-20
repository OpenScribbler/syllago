# Demo 1: TUI Browsing
# Shows launching the TUI, navigating content, and previewing items.

Output docs/vhs/demo-tui-browse.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 800
Set Padding 20
Set TypingSpeed 60ms

# Launch syllago TUI
Type "syllago"
Enter
Sleep 2s

# Navigate to Skills
Down
Sleep 500ms
Enter
Sleep 1s

# Browse skills list
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

# Open a skill detail
Enter
Sleep 1.5s

# Switch to Files tab
Tab
Sleep 1s

# Show file preview
Down
Sleep 500ms
Enter
Sleep 2s

# Go back
Escape
Sleep 1s
Escape
Sleep 1s

# Navigate to Loadouts
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Enter
Sleep 1.5s

# Show loadout detail
Enter
Sleep 2s

# Quit
Escape
Sleep 500ms
Escape
Sleep 500ms
Type "q"
Sleep 1s
