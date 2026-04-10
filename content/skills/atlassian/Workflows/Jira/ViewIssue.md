# ViewIssue Workflow

Fetch and display ATD Jira issue details in a clear, readable format.

## Process

### 1. Extract Issue Key

From user input, extract the ATD issue key:

- Format: `ATD-###` (e.g., `ATD-836`, `ATD-1234`)
- Handle variations: "ATD836", "atd-836", "ATD 836" → normalize to `ATD-836`
- If no key provided, ask user for it

### 2. Load MCP Tool

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__getJiraIssue")
```

### 3. Fetch Issue

Call the MCP tool with proper parameters:

```typescript
mcp__plugin_atlassian_atlassian__getJiraIssue({
  cloudId: "5d952b27-0223-4b75-8999-6f5dd97440be",
  issueIdOrKey: "ATD-###"
})
```

### 4. Format Output

Display the issue information in a structured format:

```markdown
## [Issue Key]: [Summary]

**Status**: [Status name]
**Type**: [Issue type]
**Priority**: [Priority]
**Assignee**: [Name or "Unassigned"]
**Reporter**: [Name]

**Created**: [Date]
**Updated**: [Date]
**Labels**: [Label list or "None"]

### Description

[Full description content]

### Links

**Issue URL**: https://aembit.atlassian.net/browse/[Issue Key]
```

### 5. Handle Related Information

If relevant, include:

- **Parent Issue**: Link to parent (for sub-tasks)
- **Sub-tasks**: List of child issues
- **Comments**: Recent comments (if requested)
- **Attachments**: List of attachments
- **Issue Links**: Related/blocked issues

## Error Handling

### Issue Not Found

If the issue doesn't exist:

```
❌ Issue ATD-### not found.

Possible reasons:
- Issue key is incorrect
- Issue is in a different project
- You don't have permission to view this issue
```

### Permission Denied

If access is restricted:

```
❌ Cannot access ATD-###

You don't have permission to view this issue. Contact your Jira administrator.
```

## Examples

### Example 1: Simple View

User: "Show me ATD-836"

1. Extract key: `ATD-836`
2. Load tool: `ToolSearch(...)`
3. Fetch issue
4. Display:
```
## ATD-836: preview.sh fails: githubFetchedContent collection not found

**Status**: To Do
**Type**: Docs Bug
**Priority**: Medium
**Assignee**: Unassigned
**Reporter**: Holden Hewett

**Created**: 2026-01-26
**Updated**: 2026-01-26
**Labels**: DocBug, Docs

### Description

## Problem

When running `preview.sh` locally on Mac, the build fails with the error:
...

**Issue URL**: https://aembit.atlassian.net/browse/ATD-836
```

### Example 2: View with Context

User: "What's the status of the preview script bug?"

1. Infer they mean ATD-836 from context
2. Fetch issue
3. Display with status emphasis: "ATD-836 is currently **To Do**"

### Example 3: Batch View

User: "Show me ATD-836, ATD-837, and ATD-838"

1. Extract all keys: `[ATD-836, ATD-837, ATD-838]`
2. Fetch each in parallel (if possible)
3. Display condensed view:
```
## ATD-836: preview.sh fails (Docs Bug, To Do)
## ATD-837: Lambda troubleshooting guide (Docs Request, In Progress)
## ATD-838: Automate screenshots (DocOps Task, To Do)
```

## Optional Expansions

### View Comments

If user requests comments:

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__getJiraIssueComments")
// Then call to fetch comments
```

Display as:

```
### Comments

**[Author] - [Date]**
[Comment text]

**[Author] - [Date]**
[Comment text]
```

### View Transitions

If user asks what actions are available:

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__getTransitionsForJiraIssue")
// Fetch available workflow transitions
```

Display as:

```
### Available Actions

- Move to In Progress
- Move to Done
- Move to Blocked
```

## Output Formatting Tips

- **Keep it scannable**: Use headers, bold, and whitespace
- **Highlight key info**: Status, assignee, and priority should stand out
- **Truncate long descriptions**: For batch views, show first 2-3 lines only
- **Include URLs**: Always provide clickable link to full issue
- **Use emoji sparingly**: Only for status indicators if helpful (⏳ In Progress, ✅ Done, 🚫 Blocked)

## Quality Checks

- [ ] Issue key is valid format
- [ ] Cloud ID is correct
- [ ] Output is formatted clearly
- [ ] Issue URL is included
- [ ] Error messages are helpful
