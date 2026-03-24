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

# Preview: same rule in Claude Code format
Type "syllago inspect rules/claude-code/coding-standards --as claude-code | head -6"
Enter
Sleep 4s

# Preview: same rule in Cursor format (notice the frontmatter)
Type "syllago inspect rules/claude-code/coding-standards --as cursor | head -8"
Enter
Sleep 5s

Sleep 3s
