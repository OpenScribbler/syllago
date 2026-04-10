# ViewPage Workflow

Fetch and display Confluence page content.

## Process

### 1. Identify Page

User can specify page by:
- **Page ID**: Direct numeric ID (e.g., "123456789")
- **Page Title**: "Show me the 'API Documentation' page"
- **Page URL**: Full Confluence URL

### 2. Load MCP Tool

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__getConfluencePage")
```

### 3. Fetch Page

```typescript
mcp__plugin_atlassian_atlassian__getConfluencePage({
  cloudId: process.env.ATLASSIAN_CLOUD_ID,
  pageId: "[page ID]"
})
```

**If page title provided** (not ID), search first:
```typescript
mcp__plugin_atlassian_atlassian__searchConfluenceUsingCql({
  cloudId: process.env.ATLASSIAN_CLOUD_ID,
  cql: `title ~ "[title]"`
})
```

### 4. Format Output

Display page information:

```markdown
## [Page Title]

**Space**: [Space Name]
**URL**: [Page URL]
**Last Modified**: [Date] by [Author]

### Content

[Page content in markdown]
```

## Examples

**Example: View by title**
```
User: "Show me the Lambda Integration Setup page"
→ Search CQL: title ~ "Lambda Integration Setup"
→ Fetch page by ID from search results
→ Display content
```
