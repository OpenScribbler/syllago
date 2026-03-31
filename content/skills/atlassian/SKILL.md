---
name: atlassian
description: Comprehensive Atlassian workspace management via MCP. USE WHEN working with Jira issues (ATD, APM, BAC, EDGE-X, any project) OR Confluence pages OR searching Atlassian OR creating tickets OR viewing issues OR updating status. Embeds project metadata to avoid long tool names.
---

# atlassian

Complete Atlassian workspace management using the Atlassian MCP server. Handles Jira issues across all projects (ATD, APM, BAC, EDGE-X, etc.) and Confluence operations (pages, spaces, search). Provides unified access to the Aembit Atlassian workspace with embedded metadata to avoid problematic long tool names.

## Atlassian Workspace

- **Cloud ID**: `$ATLASSIAN_CLOUD_ID` (environment variable - see setup below)
- **Jira URL**: https://aembit.atlassian.net/jira/
- **Confluence URL**: https://aembit.atlassian.net/wiki/

**Environment Setup:**

To use this skill, you must first obtain your Atlassian Cloud ID:

1. Load the MCP tool:
   ```
   ToolSearch("select:mcp__plugin_atlassian_atlassian__getAccessibleAtlassianResources")
   ```

2. Call the tool to get your Cloud ID:
   ```typescript
   mcp__plugin_atlassian_atlassian__getAccessibleAtlassianResources()
   ```

3. The response will include your `id` field - this is your Cloud ID.

4. Set it as an environment variable in your `~/.bashrc` or `~/.zshrc`:
   ```bash
   export ATLASSIAN_CLOUD_ID="your-cloud-id-here"
   ```

5. Reload your shell: `source ~/.bashrc`

**CRITICAL: Never call metadata tools with names longer than 64 characters.** Use embedded configurations below.

## Jira Projects

### ATD (Aembit Technical Documentation)

**Project Key**: `ATD`
**Board URL**: https://aembit.atlassian.net/jira/software/projects/ATD/boards/6

#### Issue Types

| Issue Type | Work Type ID | Use When | Default Labels |
|------------|--------------|----------|----------------|
| **Docs Bug** | `10056` | Defects in existing documentation (typos, broken links, incorrect steps) | `["DocBug", "Docs"]` |
| **Docs Request** (trailing space!) | `10045` | New documentation, enhancements, troubleshooting guides, feature docs | `["Docs"]` |
| **Maverick Task** | `10094` | 1C/Maverick chip tasks with templates (Task Size, Requirements, Acceptance Criteria) | `["Docs"]` |
| **DocOps Task** | `10072` | Documentation infrastructure, automation, pipelines, tooling | `["DocOps"]` |

**IMPORTANT**: "Docs Request " has a trailing space in the name.

#### Priorities

- `High` - Use only when clearly urgent
- `Medium` - Default for most work

### APM (Aembit Product Management)

**Project Key**: `APM`
**Use**: Product management initiatives, feature planning, roadmap items, main project work

*Issue type metadata: Fetch dynamically or use standard Jira types (Story, Task, Bug, Epic)*

### BAC (Business Application/Context)

**Project Key**: `BAC`
**Use**: Business application tickets, product requirements, cross-team coordination

*Issue type metadata: Fetch dynamically or use standard Jira types (Story, Task, Bug)*

### EDGE-X Projects

**Project Pattern**: `EDGE-*`
**Use**: Edge component releases, package management, technical infrastructure

*Issue type metadata: Fetch dynamically or use standard types*

### Generic Jira Operations

For projects without embedded metadata, use standard Jira issue types:
- `Story` - Feature work
- `Task` - General work item
- `Bug` - Defects
- `Epic` - Large initiatives

## Confluence Spaces

Common Confluence spaces in the Aembit workspace:

- **Engineering** - Technical specs, architecture docs
- **Product** - Product requirements, roadmaps
- **Documentation** - Documentation guidelines, style guides
- **Operations** - Runbooks, incident reports

*Use `searchConfluenceUsingCql` or `getConfluenceSpaces` to discover available spaces*

## Workflow Routing

### Jira Workflows

| Workflow | Trigger | File |
|----------|---------|------|
| **CreateIssue** | "create [PROJECT] issue" OR "file a bug" OR "new ticket" | `Workflows/Jira/CreateIssue.md` |
| **ViewIssue** | "show [KEY]" OR "get issue details" OR "view [PROJECT]-###" | `Workflows/Jira/ViewIssue.md` |
| **SearchIssues** | "find issues" OR "search Jira" OR "list open bugs in [PROJECT]" | `Workflows/Jira/SearchIssues.md` |
| **UpdateIssue** | "update [KEY]" OR "change status" OR "assign to" OR "add comment" | `Workflows/Jira/UpdateIssue.md` |

### Confluence Workflows

| Workflow | Trigger | File |
|----------|---------|------|
| **ViewPage** | "show Confluence page" OR "get page [title]" | `Workflows/Confluence/ViewPage.md` |
| **SearchPages** | "search Confluence" OR "find pages about [topic]" | `Workflows/Confluence/SearchPages.md` |
| **CreatePage** | "create Confluence page" OR "new page in [space]" | `Workflows/Confluence/CreatePage.md` |
| **UpdatePage** | "update Confluence page" OR "edit page [title]" | `Workflows/Confluence/UpdatePage.md` |

### Cross-Platform Workflows

| Workflow | Trigger | File |
|----------|---------|------|
| **SearchAtlassian** | "search Atlassian" OR "find across Jira and Confluence" | `Workflows/SearchAtlassian.md` |

## Examples

### Jira Examples

**Example 1: Create ATD documentation bug**
```
User: "Create an ATD issue for the broken link on the quickstart page"
→ Invokes CreateIssue workflow
→ Detects project: ATD, type: Docs Bug
→ Uses ATD metadata (Docs Bug issue type, DocBug + Docs labels)
→ Creates structured issue with Problem/Root Cause/Fix
```

**Example 2: View APM feature**
```
User: "Show me APM-456"
→ Invokes ViewIssue workflow
→ Fetches APM-456 using getJiraIssue
→ Displays feature details, status, assignee
```

**Example 3: View BAC ticket**
```
User: "Show me BAC-123"
→ Invokes ViewIssue workflow
→ Fetches BAC-123 using getJiraIssue
→ Displays summary, description, status, assignee
```

**Example 4: Search EDGE-X releases**
```
User: "Find all open EDGE-X issues for the 1.28 release"
→ Invokes SearchIssues workflow
→ Constructs JQL: project = "EDGE-X" AND fixVersion = "1.28" AND status != Done
→ Returns matching issues
```

**Example 5: Update cross-project**
```
User: "Move BAC-456 to In Progress and add a comment"
→ Invokes UpdateIssue workflow
→ Transitions status, adds comment using ADF format
→ Confirms changes
```

### Confluence Examples

**Example 6: Search for architecture docs**
```
User: "Find Confluence pages about microservices architecture"
→ Invokes SearchPages workflow
→ Uses searchConfluenceUsingCql with text search
→ Returns relevant pages with excerpts
```

**Example 7: View specific page**
```
User: "Show me the API documentation page from the Engineering space"
→ Invokes ViewPage workflow
→ Searches for page by title in Engineering space
→ Displays page content
```

**Example 8: Create meeting notes**
```
User: "Create a Confluence page in the Engineering space for today's standup notes"
→ Invokes CreatePage workflow
→ Prompts for page title and content
→ Creates page with proper structure
```

### Cross-Platform Examples

**Example 9: Search everything**
```
User: "Search Atlassian for information about the Lambda integration"
→ Invokes SearchAtlassian workflow
→ Searches both Jira and Confluence
→ Returns issues and pages ranked by relevance
```

## MCP Tools Reference

### Jira Tools (loaded via ToolSearch)

- `mcp__plugin_atlassian_atlassian__getJiraIssue` - Fetch issue by key
- `mcp__plugin_atlassian_atlassian__createJiraIssue` - Create new issue
- `mcp__plugin_atlassian_atlassian__editJiraIssue` - Update existing issue
- `mcp__plugin_atlassian_atlassian__searchJiraIssuesUsingJql` - Search with JQL
- `mcp__plugin_atlassian_atlassian__addCommentToJiraIssue` - Add comment
- `mcp__plugin_atlassian_atlassian__transitionJiraIssue` - Change workflow state
- `mcp__plugin_atlassian_atlassian__getTransitionsForJiraIssue` - Get available transitions
- `mcp__plugin_atlassian_atlassian__lookupJiraAccountId` - Find user account ID
- `mcp__plugin_atlassian_atlassian__getVisibleJiraProjects` - List accessible projects

### Confluence Tools (loaded via ToolSearch)

- `mcp__plugin_atlassian_atlassian__getConfluencePage` - Fetch page by ID
- `mcp__plugin_atlassian_atlassian__searchConfluenceUsingCql` - Search with CQL
- `mcp__plugin_atlassian_atlassian__getConfluenceSpaces` - List spaces
- `mcp__plugin_atlassian_atlassian__getPagesInConfluenceSpace` - List pages in space
- `mcp__plugin_atlassian_atlassian__createConfluencePage` - Create new page
- `mcp__plugin_atlassian_atlassian__updateConfluencePage` - Update page content
- `mcp__plugin_atlassian_atlassian__getConfluencePageFooterComments` - Get comments
- `mcp__plugin_atlassian_atlassian__createConfluenceFooterComment` - Add comment

### Cross-Platform Tools

- `mcp__plugin_atlassian_atlassian__search` - Unified search across Jira and Confluence
- `mcp__plugin_atlassian_atlassian__fetch` - Generic fetch operation

**Tool name limitation**: Names exceeding 64 characters cannot be called. This is why we embed static metadata.

## Project Selection Logic

When user mentions a project:

1. **Explicit project key**: "Create BAC issue", "Show ATD-836", "Search EDGE-X"
   → Use specified project

2. **Context clues**: "documentation bug", "docs request", "chip task"
   → Likely ATD project (but confirm if ambiguous)

3. **No project specified**: "Create a bug", "Search for issues"
   → Ask user which project to use

4. **Cross-project search**: "Find all issues about Lambda"
   → Search across all projects (no project filter in JQL)

## ATD-Specific Templates

*See full templates in `Workflows/Jira/CreateIssue.md`*

### Docs Bug Template
```markdown
## Problem
[Description of defect]

## Root Cause
[Why this happened]

## Proposed Fix
[How to fix]

## Technical Details
- **File**: [path]
- **Line**: [number]

## Acceptance Criteria
- [ ] [Fix applied]
- [ ] [Verified]
```

### Docs Request Template
```markdown
## Summary
[Brief description]

## Goal
[What this should accomplish]

## Technical requirements & details
[Specifics]

## Acceptance Criteria
- [ ] [Deliverable]

## Links & Related
[References]
```

## Configuration Discovery

For projects without embedded metadata:

### Discover Projects
```typescript
mcp__plugin_atlassian_atlassian__getVisibleJiraProjects({
  cloudId: process.env.ATLASSIAN_CLOUD_ID
})
```

### Discover Confluence Spaces
```typescript
mcp__plugin_atlassian_atlassian__getConfluenceSpaces({
  cloudId: process.env.ATLASSIAN_CLOUD_ID
})
```

### Dynamic Issue Type Discovery
*Not recommended due to long tool names, but available if needed*

Use JQL search to examine existing issues and infer types, or ask user to specify type name directly.

## Quality Standards

- Always use Cloud ID from environment: `process.env.ATLASSIAN_CLOUD_ID`
- Verify `ATLASSIAN_CLOUD_ID` is set before MCP calls
- For ATD issues: Use embedded issue type metadata
- For other projects: Use standard types or ask user
- Include issue/page URLs in responses
- Format JQL/CQL queries for user visibility
- Provide clear error messages with actionable guidance
