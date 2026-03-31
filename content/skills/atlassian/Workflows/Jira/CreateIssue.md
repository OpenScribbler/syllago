# CreateIssue Workflow

Create ATD Jira issues with smart issue type detection and proper field mapping.

## Process

### 1. Analyze Intent

Determine the appropriate issue type based on the user's request:

| User Intent | Issue Type | Signals |
|-------------|------------|---------|
| **Docs Bug** | `Docs Bug` | "bug", "broken", "typo", "incorrect", "wrong", "error", "fix", "defect" |
| **Docs Request** | `Docs Request ` | "document", "add docs", "create guide", "need docs", "enhancement", "troubleshooting" |
| **Maverick Task** | `Maverick Task` | "chip", "1C", "maverick", "task size", "conceptual work" |
| **DocOps Task** | `DocOps Task` | "pipeline", "automation", "CI/CD", "tooling", "infrastructure", "repo setup" |

**Default**: If unclear, ask the user or default to "Docs Request ".

### 2. Load MCP Tool

Before calling any Atlassian MCP tool, load it first:

```
ToolSearch("select:mcp__plugin_atlassian_atlassian__createJiraIssue")
```

### 3. Prepare Issue Fields

**Required fields:**
- `cloudId`: `5d952b27-0223-4b75-8999-6f5dd97440be`
- `projectKey`: `ATD`
- `issueTypeName`: One of the four issue types (see Static Metadata)
- `summary`: Clear, concise title (1-2 lines)
- `description`: Structured markdown (see templates in SKILL.md)

**Optional fields** (if supported by the MCP tool signature):
- `priority`: Default to `Medium`
- `labels`: Apply appropriate labels based on issue type
- `assignee`: User's account ID (if specified)

**IMPORTANT**: The "Docs Request " issue type has a **trailing space** in the name. Always include it: `"Docs Request "` not `"Docs Request"`.

### 4. Select Template

Use the appropriate description template from SKILL.md:

- **Docs Bug**: Problem → Root Cause → Proposed Fix → Technical Details → Acceptance Criteria
- **Docs Request**: Summary → Goal → Technical Requirements → Acceptance Criteria → Links
- **Maverick Task**: Task Size → Description → Requirements → Acceptance Criteria → Bonus Opportunities
- **DocOps Task**: Summary → Technical Requirements → Acceptance Criteria → Links

### 5. Create Issue

Call the MCP create tool:

```typescript
mcp__plugin_atlassian_atlassian__createJiraIssue({
  cloudId: "5d952b27-0223-4b75-8999-6f5dd97440be",
  projectKey: "ATD",
  issueTypeName: "[Selected Type]",
  summary: "[Issue title]",
  description: "[Formatted markdown description]"
  // Optional: priority, labels, assignee
})
```

### 6. Handle Response

- **Success**: Display the created issue key (e.g., "Created ATD-837") and provide the issue URL
- **Error**: If you get "Specify a valid issue type":
  - Double-check the issue type name (especially the trailing space in "Docs Request ")
  - Try the alternative type based on intent
  - Never guess random issue type names

## Error Recovery

### Issue Type Not Found

If the MCP tool rejects the issue type name:

1. Verify you used the exact name with proper spacing
2. For Docs Request, ensure trailing space: `"Docs Request "`
3. Fall back to "Docs Bug" for defects or "Docs Request " for requests
4. If all fail, report to user with exact error message

### Missing Fields

If required fields are missing:

1. Prompt user for missing information
2. Use AskUserQuestion for clarity
3. Don't create partial/incomplete issues

## Examples

### Example 1: Documentation Bug

User says: "The preview.sh script fails with a missing collection error"

1. Detect intent: "fails", "error" → **Docs Bug**
2. Load tool: `ToolSearch("select:mcp__plugin_atlassian_atlassian__createJiraIssue")`
3. Prepare fields:
   ```
   issueTypeName: "Docs Bug"
   summary: "preview.sh fails: githubFetchedContent collection not found"
   description: "## Problem\n\nWhen running `preview.sh`..."
   ```
4. Create issue
5. Return: "Created ATD-836: https://aembit.atlassian.net/browse/ATD-836"

### Example 2: Documentation Request

User says: "We need troubleshooting docs for the Lambda integration"

1. Detect intent: "need", "docs" → **Docs Request**
2. Load tool
3. Prepare fields:
   ```
   issueTypeName: "Docs Request "  // Note trailing space
   summary: "Create troubleshooting guide for Lambda integration"
   description: "## Summary\n\nDevelopers need..."
   ```
4. Create issue
5. Return: "Created ATD-838: ..."

### Example 3: DocOps Task

User says: "Set up automated screenshot updates in the CI pipeline"

1. Detect intent: "pipeline", "automated" → **DocOps Task**
2. Load tool
3. Prepare fields:
   ```
   issueTypeName: "DocOps Task"
   summary: "Automate screenshot updates in CI pipeline"
   description: "## Summary\n\nIntegrate screenshot automation..."
   labels: ["DocOps"]
   ```
4. Create issue
5. Return: "Created ATD-839: ..."

## Quality Checks

Before creating the issue, verify:

- [ ] Issue type matches user intent
- [ ] Summary is clear and specific (not vague)
- [ ] Description uses appropriate template structure
- [ ] Trailing space included for "Docs Request " type
- [ ] Cloud ID and project key are correct
- [ ] Priority is appropriate (default Medium)

## User Confirmation

For complex issues or when intent is unclear, use AskUserQuestion to confirm:

```
Question: "What type of issue should I create?"
Options:
- "Docs Bug - Fix a defect in existing documentation"
- "Docs Request - New or enhanced documentation"
- "Maverick Task - 1C chip documentation work"
- "DocOps Task - Infrastructure or tooling work"
```
