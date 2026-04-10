# SearchPages Workflow

Search Confluence pages using CQL (Confluence Query Language).

## Process

### 1. Build CQL Query

Translate natural language to CQL:

| User Intent | CQL Query |
|-------------|-----------|
| "Search for Lambda" | `text ~ "Lambda"` |
| "Pages in Engineering space" | `space = "Engineering"` |
| "Pages by John" | `creator = "[accountId]"` |
| "Updated this week" | `lastModified >= now("-7d")` |

### 2. Execute Search

```typescript
mcp__plugin_atlassian_atlassian__searchConfluenceUsingCql({
  cloudId: process.env.ATLASSIAN_CLOUD_ID,
  cql: "[constructed query]"
})
```

### 3. Format Results

```markdown
Found [N] pages:

1. **[Title]** - [Space] ([URL])
   [Excerpt...]

2. **[Title]** - [Space] ([URL])
   [Excerpt...]
```

## CQL Patterns

```cql
# Text search
text ~ "search term"

# Space filter
space = "ENG"

# Type filter
type = "page"

# Date filter
lastModified >= "2026-01-01"

# Combined
text ~ "Lambda" AND space = "Engineering" AND type = "page"
```
