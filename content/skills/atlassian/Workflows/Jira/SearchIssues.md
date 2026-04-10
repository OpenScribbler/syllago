# SearchIssues Workflow

Search ATD Jira issues using JQL (Jira Query Language) with smart query construction.

## Process

### 1. Understand Search Intent

Parse the user's natural language request into structured search criteria:

| User Intent | JQL Component | Example |
|-------------|---------------|---------|
| "open issues" | `status != Done` | status in (To Do, In Progress) |
| "assigned to me" | `assignee = currentUser()` | assignee = currentUser() |
| "high priority" | `priority = High` | priority = High |
| "docs bugs" | `type = "Docs Bug"` | type = "Docs Bug" |
| "created this week" | `created >= -7d` | created >= startOfWeek() |
| "unassigned" | `assignee is EMPTY` | assignee is EMPTY |

### 2. Build JQL Query

Construct JQL with these components:

**Base filter** (always include):
```jql
project = ATD
```

**Additional filters** (combine with AND):
- Issue type: `AND type = "Docs Bug"`
- Status: `AND status = "In Progress"`
- Priority: `AND priority = High`
- Assignee: `AND assignee = currentUser()`
- Labels: `AND labels = DocOps`
- Date range: `AND created >= -30d`

**Order by** (optional, default to updated):
```jql
ORDER BY updated DESC
```

**Note**: Use double quotes for values with spaces: `type = "Docs Request "` (includes trailing space!)

### 3. Load MCP Tool

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__searchJiraIssuesUsingJql")
```

### 4. Execute Search

Call the MCP tool:

```typescript
mcp__plugin_atlassian_atlassian__searchJiraIssuesUsingJql({
  cloudId: "5d952b27-0223-4b75-8999-6f5dd97440be",
  jql: "[constructed query]",
  maxResults: 50  // Adjust based on need
})
```

### 5. Format Results

Display results in a scannable format:

**Summary view** (default):
```markdown
Found [N] issues matching your search:

1. **ATD-###** - [Summary] ([Type], [Status])
2. **ATD-###** - [Summary] ([Type], [Status])
...

[Show JQL query used]
```

**Detailed view** (if requested):
```markdown
## ATD-###: [Summary]
**Type**: [Type] | **Status**: [Status] | **Priority**: [Priority]
**Assignee**: [Name] | **Updated**: [Date]

[First 200 chars of description...]

---
```

## Common JQL Patterns

### By Status

```jql
# All open issues
project = ATD AND status != Done

# Specific status
project = ATD AND status = "In Progress"

# Multiple statuses
project = ATD AND status in ("To Do", "In Progress")
```

### By Type

```jql
# All Docs Bugs
project = ATD AND type = "Docs Bug"

# All Docs Requests (note trailing space!)
project = ATD AND type = "Docs Request "

# DocOps work
project = ATD AND type = "DocOps Task"
```

### By Assignee

```jql
# My issues
project = ATD AND assignee = currentUser()

# Unassigned issues
project = ATD AND assignee is EMPTY

# Specific person (need account ID)
project = ATD AND assignee = "712020:dd83af54-d205-43ce-83dc-1d167cb6b678"
```

### By Date

```jql
# Created in last 7 days
project = ATD AND created >= -7d

# Updated today
project = ATD AND updated >= startOfDay()

# Created this month
project = ATD AND created >= startOfMonth()
```

### By Priority

```jql
# High priority only
project = ATD AND priority = High

# Medium or High
project = ATD AND priority in (Medium, High)
```

### By Labels

```jql
# DocOps work
project = ATD AND labels = DocOps

# Documentation bugs
project = ATD AND labels in (DocBug, Docs)
```

### Combined Queries

```jql
# Open Docs Bugs assigned to me
project = ATD AND type = "Docs Bug" AND status != Done AND assignee = currentUser()

# Unassigned high priority requests
project = ATD AND type = "Docs Request " AND priority = High AND assignee is EMPTY

# Recent DocOps tasks
project = ATD AND type = "DocOps Task" AND created >= -30d ORDER BY created DESC
```

## Examples

### Example 1: Find My Open Issues

User: "Show me my open ATD issues"

1. Intent: My issues + not done
2. JQL: `project = ATD AND assignee = currentUser() AND status != Done`
3. Execute search
4. Display:
```
Found 3 issues assigned to you:

1. **ATD-836** - preview.sh fails: githubFetchedContent collection not found (Docs Bug, To Do)
2. **ATD-837** - Create Lambda troubleshooting guide (Docs Request, In Progress)
3. **ATD-840** - Update API reference formatting (Docs Request, In Progress)
```

### Example 2: Find Unassigned Bugs

User: "List all unassigned Docs Bugs in ATD"

1. Intent: Docs Bug + unassigned
2. JQL: `project = ATD AND type = "Docs Bug" AND assignee is EMPTY`
3. Execute search
4. Display results with count

### Example 3: Recent High Priority Work

User: "What high priority issues were created this week?"

1. Intent: High priority + created recently
2. JQL: `project = ATD AND priority = High AND created >= startOfWeek() ORDER BY created DESC`
3. Execute search
4. Display with dates

### Example 4: DocOps Backlog

User: "Show me the DocOps backlog"

1. Intent: DocOps tasks + not done
2. JQL: `project = ATD AND type = "DocOps Task" AND status = "To Do" ORDER BY priority DESC`
3. Execute search
4. Display ordered by priority

## Error Handling

### Invalid JQL Syntax

If JQL is rejected:

```
❌ Search failed: Invalid JQL syntax

The query was: [show query]

Common issues:
- Missing quotes around values with spaces
- Invalid field names
- Wrong date format
```

### No Results Found

```
No issues found matching:
[Show JQL query]

Try:
- Broadening your search criteria
- Checking status filters
- Verifying issue type names
```

### Too Many Results

If more than 100 results:

```
Found [N] issues (showing first 50)

[Results...]

Refine your search to see all results:
- Add date range: created >= -30d
- Filter by status: status = "To Do"
- Filter by type: type = "Docs Bug"
```

## User Assistance

### Show JQL Query

Always display the constructed JQL so users can:
- Learn JQL syntax
- Copy query to Jira directly
- Understand what was searched
- Modify for custom searches

Format:
```
📋 JQL Query:
project = ATD AND status != Done AND assignee = currentUser()

🔗 Run in Jira:
https://aembit.atlassian.net/issues/?jql=project+%3D+ATD+AND+status+%21%3D+Done
```

### Suggest Refinements

If search returns too many/few results, suggest:

```
💡 Suggestions:
- Add date filter: created >= -7d
- Filter by priority: AND priority = High
- Exclude type: AND type != "DocOps Task"
```

## Quality Checks

- [ ] JQL syntax is valid
- [ ] Project = ATD is always included
- [ ] Quotes used for values with spaces
- [ ] Trailing space included for "Docs Request "
- [ ] Order by clause appropriate
- [ ] Results formatted clearly
- [ ] Query shown to user
