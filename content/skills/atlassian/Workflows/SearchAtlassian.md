# SearchAtlassian Workflow

Search across both Jira and Confluence in a single query.

## Process

### 1. Load MCP Tool

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__search")
```

### 2. Execute Unified Search

```typescript
mcp__plugin_atlassian_atlassian__search({
  cloudId: process.env.ATLASSIAN_CLOUD_ID,
  query: "[search term]"
})
```

This searches across:
- **Jira issues** (all projects)
- **Confluence pages** (all spaces)
- **Confluence blog posts**
- **People** (users)

### 3. Format Results

Group results by type:

```markdown
## Search Results for "[query]"

### Jira Issues ([count])

1. **[KEY]** - [Summary] ([Project], [Status])
2. **[KEY]** - [Summary] ([Project], [Status])

### Confluence Pages ([count])

1. **[Title]** - [Space] ([URL])
2. **[Title]** - [Space] ([URL])

### People ([count])

1. **[Name]** ([email]) - [role]
```

## Examples

**Example 1: Search for Lambda content**
```
User: "Search Atlassian for Lambda integration"
→ Unified search: query = "Lambda integration"
→ Returns:
  - Jira: BAC-123, ATD-456 (issues about Lambda)
  - Confluence: Setup guides, troubleshooting pages
→ Display grouped by type
```

**Example 2: Find all mentions of a feature**
```
User: "Find everything related to the new authentication flow"
→ Search: "authentication flow"
→ Returns issues tracking the work + docs pages explaining it
→ User can see full context across platforms
```

## When to Use

Use this workflow when:
- User wants to search "everywhere"
- Topic spans both Jira and Confluence
- Need to find all mentions of a feature/topic

Use specific workflows (SearchIssues, SearchPages) when:
- User only wants Jira or Confluence results
- Need advanced filtering (JQL/CQL)
- Need to construct complex queries

## Quality Tips

- **Highlight relevance**: Show why each result matches
- **Group by type**: Keep Jira and Confluence results separate
- **Limit results**: Show top 10-20, offer to refine
- **Provide URLs**: Make all results clickable
