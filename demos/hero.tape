# Hero demo: TUI browsing library and loadouts
#
# Shows: launch -> browse library -> view loadout details
#
# Prerequisites:
#   1. Ensure library has content: syllago list
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

# Launch syllago TUI
Type "syllago"
Enter
Sleep 3s

# Pause on homepage — shows Library with content cards
Sleep 3s

# Browse items — arrow down through a few
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s

# Select an item to see detail view
Enter
Sleep 3s

# Back out
Escape
Sleep 1s

# Navigate to Loadouts in sidebar
# Sidebar: Library (selected), Loadouts, Registries, Settings
# Tab to sidebar, then Down to Loadouts
Down
Sleep 400ms
Down
Sleep 400ms
Enter
Sleep 3s

# Browse loadout cards
Down
Sleep 1s
Down
Sleep 1s

# Select a loadout to see its contents
Enter
Sleep 3s

# Scroll down to see more detail
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s

# Pause on loadout detail
Sleep 3s

# Quit
Type "q"
Sleep 1s
