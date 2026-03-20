# Demo 2: Loadout Apply
# Shows applying the syllago-starter loadout and verifying the result.

Output docs/vhs/demo-loadout-apply.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 800
Set Padding 20
Set TypingSpeed 60ms

# Preview the loadout
Type "syllago loadout apply syllago-starter"
Enter
Sleep 3s

# Apply permanently
Type "syllago loadout apply syllago-starter --keep"
Enter
Sleep 3s

# Verify installed content
Type "syllago list --type skills"
Enter
Sleep 2s

# Show loadout status
Type "syllago loadout status"
Enter
Sleep 3s

# Clean up - remove the loadout
Type "syllago loadout remove"
Enter
Sleep 2s
