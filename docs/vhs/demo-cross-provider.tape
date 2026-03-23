# Demo 3: Cross-Provider Conversion
# Shows converting content between providers with automatic format translation.

Output docs/vhs/demo-cross-provider.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 800
Set Padding 20
Set TypingSpeed 60ms

# Show what's in the library
Type "syllago list --type skills"
Enter
Sleep 2s

# Inspect a skill
Type "syllago inspect skills/syllago-guide"
Enter
Sleep 3s

# Convert to Gemini CLI format
Type "syllago convert skills/syllago-guide --to gemini-cli"
Enter
Sleep 3s

# Convert to Cursor format
Type "syllago convert rules/example-kitchen-sink-rules --to cursor"
Enter
Sleep 3s

# Install to a provider
Type "syllago install skills/syllago-guide --to gemini-cli"
Enter
Sleep 3s
