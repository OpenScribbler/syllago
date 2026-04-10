# CreatePage Workflow

Create new Confluence pages with proper structure.

## Process

### 1. Gather Information

Required fields:
- **Space**: Which Confluence space (Engineering, Product, etc.)
- **Title**: Page title
- **Content**: Page content in Confluence storage format

Optional:
- **Parent Page**: ID of parent page (for hierarchy)

### 2. Load MCP Tool

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__createConfluencePage")
```

### 3. Convert Content

Confluence uses "storage format" (XHTML-like). Convert markdown to storage format or use simple HTML:

```html
<p>This is a paragraph.</p>
<h2>This is a heading</h2>
<ul>
  <li>List item 1</li>
  <li>List item 2</li>
</ul>
```

### 4. Create Page

```typescript
mcp__plugin_atlassian_atlassian__createConfluencePage({
  cloudId: process.env.ATLASSIAN_CLOUD_ID,
  spaceKey: "[SPACE]",
  title: "[Title]",
  body: {
    storage: {
      value: "[content in storage format]",
      representation: "storage"
    }
  }
})
```

### 5. Confirm Creation

Return page URL and ID:

```
Created page: [Title]
URL: https://aembit.atlassian.net/wiki/spaces/[SPACE]/pages/[ID]/[slug]
```

## Examples

**Example: Create meeting notes**
```
User: "Create a Confluence page in Engineering for today's standup notes"
→ Prompt for title: "Daily Standup - 2026-01-26"
→ Prompt for content or use template
→ Create page with proper storage format
→ Return URL
```
