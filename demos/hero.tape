# Hero demo: TUI walkthrough showing syllago's interactive interface
#
# Shows: launch -> homepage -> Library -> browse content -> item detail
#
# Prerequisites:
#   1. cd ~/demo-project (or any project with syllago content)
#   2. Ensure library has content: syllago list
#
# Usage: vhs hero.tape (run from ~/demo-project)

Output "hero.gif"

Set Shell "bash"
Set FontSize 15
Set Width 1200
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

# Launch syllago TUI
Type "syllago"
Enter
Sleep 2.5s

# Show the homepage with cards
Sleep 1.5s

# Navigate to Library
Down
Sleep 300ms
Enter
Sleep 1.5s

# Browse content type cards
Sleep 1s
Down
Sleep 300ms
Down
Sleep 300ms

# Enter a content type (Skills)
Enter
Sleep 1.5s

# Browse items
Down
Sleep 400ms
Down
Sleep 400ms

# Open an item to see detail
Enter
Sleep 2s

# Scroll through the content
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms

Sleep 2s

# Search demo
Escape
Sleep 500ms
Type "/"
Sleep 500ms
Type "guide"
Sleep 1.5s

# Exit search
Escape
Sleep 500ms

# Back out
Escape
Sleep 500ms
Escape
Sleep 500ms

# Quit
Type "q"
Sleep 1s
