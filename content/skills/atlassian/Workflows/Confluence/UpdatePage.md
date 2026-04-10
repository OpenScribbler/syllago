# UpdatePage Workflow

Update existing Confluence page content.

## Process

### 1. Identify Page

Get page ID from:
- User provides ID directly
- Search by title to find ID

### 2. Load MCP Tools

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__getConfluencePage")
ToolSearch("select:mcp__plugin_atlassian_atlassian__updateConfluencePage")
```

### 3. Fetch Current Content

Always fetch current version first:

```typescript
mcp__plugin_atlassian_atlassian__getConfluencePage({
  cloudId: process.env.ATLASSIAN_CLOUD_ID,
  pageId: "[pageId]"
})
```

**Why?** Need current version number for update (Confluence version control).

### 4. Update Page

```typescript
mcp__plugin_atlassian_atlassian__updateConfluencePage({
  cloudId: process.env.ATLASSIAN_CLOUD_ID,
  pageId: "[pageId]",
  title: "[same or new title]",
  body: {
    storage: {
      value: "[updated content]",
      representation: "storage"
    }
  },
  version: {
    number: [currentVersion + 1]
  }
})
```

**Version increment**: Always increment from current version.

### 5. Confirm Update

```
Updated page: [Title]
Version: [old] → [new]
URL: [page URL]
```

## Examples

**Example: Update content**
```
User: "Update the API docs page to include the new endpoints"
→ Search for "API Documentation" page
→ Fetch current content and version
→ Modify content (add new section)
→ Update with version number incremented
→ Confirm update
```

## Error Handling

**Version conflict**: If another user updated between fetch and update:
```
❌ Version conflict - page was updated by another user

Please retry the update. Fetching latest version...
```
