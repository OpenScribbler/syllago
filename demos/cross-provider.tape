# Cross-provider demo: add content from Claude Code, install to Cursor
#
# Prerequisites:
#   1. cd ~/demo-project
#   2. Hide global hooks (avoids name collisions with project hooks):
#      mv ~/.claude/settings.json ~/.claude/settings.json.bak
#   3. Clean library of previous test data:
#      rm -r ~/.syllago/content/rules/claude-code 2>/dev/null
#      rm -r ~/.syllago/content/skills/deploy 2>/dev/null
#      rm -r ~/.syllago/content/agents/code-reviewer 2>/dev/null
#      rm -r ~/.syllago/content/commands/claude-code 2>/dev/null
#      rm -f ~/.cursor/rule.mdc 2>/dev/null
#   4. Record: vhs cross-provider.tape
#   5. Restore: mv ~/.claude/settings.json.bak ~/.claude/settings.json

Output "cross-provider.gif"

Set Shell "bash"
Set FontSize 15
Set Width 1000
Set Height 500
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

# Add all content from Claude Code to the library
Type "syllago add --all --from claude-code"
Enter
Sleep 4.5s

# Install a rule to Cursor (auto-converts to .mdc format)
Type "syllago install coding-standards --to cursor"
Enter
Sleep 4s

# Show the same rule in Claude Code format vs Cursor format
Type "# Preview: same rule, two formats"
Enter
Sleep 1s
Type "syllago convert coding-standards --to claude-code | head -4"
Enter
Sleep 3.5s
Type "syllago convert coding-standards --to cursor | head -4"
Enter
Sleep 4.5s

Sleep 3s
