# Hero demo: TUI loadout creation wizard
#
# Shows: launch -> Loadouts -> Create -> wizard steps -> review
#
# Prerequisites:
#   1. cd ~/demo-project
#   2. Hide global settings: mv ~/.claude/settings.json ~/.claude/settings.json.bak
#   3. Ensure library has content from demo-project
#
# NOTE: Navigation depends on TUI layout. Test and adjust key counts.

Output "hero.gif"

Set Shell "bash"
Set FontSize 15
Set Width 1200
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

# Launch syllago TUI from demo project
Type "cd ~/demo-project && syllago"
Enter
Sleep 3s

# Pause on homepage
Sleep 3s

# Navigate to Loadouts (sidebar)
# Library is selected by default, Down twice to Loadouts
Down
Sleep 400ms
Down
Sleep 400ms
Enter
Sleep 3s

# Press 'c' to create a new loadout
Type "c"
Sleep 3s

# Step 1: Pick provider - select Claude Code (should be first)
Enter
Sleep 3s

# Step 2: Select content types - check Rules and Skills
Space
Sleep 400ms
Down
Sleep 400ms
Space
Sleep 400ms
Enter
Sleep 3s

# Step 3a: Select items for first type
Space
Sleep 400ms
Enter
Sleep 3s

# Step 3b: Select items for second type
Space
Sleep 400ms
Enter
Sleep 3s

# Step 4: Name the loadout
Type "my-team-config"
Sleep 1s
Tab
Sleep 300ms
Type "Standard config for the engineering team"
Sleep 1s
Enter
Sleep 3s

# Step 5: Destination
Enter
Sleep 3s

# Step 6: Review - show the summary
Sleep 3s

# Back out and quit
Escape
Sleep 500ms
Escape
Sleep 500ms
Escape
Sleep 500ms

Type "q"
Sleep 1s
