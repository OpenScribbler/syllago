# UpdateIssue Workflow

Update ATD Jira issues including status transitions, field updates, and comments.

## Process

### 1. Identify Issue and Changes

Extract from user request:

- **Issue key**: `ATD-###`
- **What to update**: Status, assignee, priority, description, labels, etc.
- **New values**: The target state or content

### 2. Determine Update Type

| Update Type | MCP Tool | Use When |
|-------------|----------|----------|
| **Field update** | `editJiraIssue` | Changing summary, description, priority, labels, assignee |
| **Status change** | `transitionJiraIssue` | Moving through workflow (To Do → In Progress → Done) |
| **Add comment** | `addCommentToJiraIssue` | Adding notes or updates without changing fields |

### 3. Load Appropriate Tool

```
# For field updates
ToolSearch("select:mcp__plugin_atlassian_atlassian__editJiraIssue")

# For status transitions
ToolSearch("select:mcp__plugin_atlassian_atlassian__transitionJiraIssue")
ToolSearch("select:mcp__plugin_atlassian_atlassian__getTransitionsForJiraIssue")

# For comments
ToolSearch("select:mcp__plugin_atlassian_atlassian__addCommentToJiraIssue")

# For account lookup
ToolSearch("select:mcp__plugin_atlassian_atlassian__lookupJiraAccountId")
```

## Field Updates

### Update Summary, Description, Priority, Labels

Use `editJiraIssue`:

```typescript
mcp__plugin_atlassian_atlassian__editJiraIssue({
  cloudId: "5d952b27-0223-4b75-8999-6f5dd97440be",
  issueIdOrKey: "ATD-###",
  update: {
    summary: [{ set: "New summary text" }],
    description: [{ set: "New description markdown" }],
    priority: [{ set: { name: "High" } }],
    labels: [{ add: "NewLabel" }]  // or { set: ["Label1", "Label2"] }
  }
})
```

**Supported field operations:**
- `set`: Replace entire value
- `add`: Add to existing (for multi-value fields like labels)
- `remove`: Remove from existing

### Update Assignee

Two approaches:

**1. Assign to self:**
```typescript
update: {
  assignee: [{ set: { accountId: "currentUser" } }]
}
```

**2. Assign to specific user:**

First lookup account ID:
```typescript
mcp__plugin_atlassian_atlassian__lookupJiraAccountId({
  cloudId: "5d952b27-0223-4b75-8999-6f5dd97440be",
  query: "user@example.com"  // or display name
})
```

Then update:
```typescript
update: {
  assignee: [{ set: { accountId: "712020:..." } }]
}
```

**3. Unassign:**
```typescript
update: {
  assignee: [{ set: null }]
}
```

## Status Transitions

### Get Available Transitions

First, check what transitions are available:

```typescript
mcp__plugin_atlassian_atlassian__getTransitionsForJiraIssue({
  cloudId: "5d952b27-0223-4b75-8999-6f5dd97440be",
  issueIdOrKey: "ATD-###"
})
```

This returns available transitions with their IDs, for example:
```json
[
  { "id": "11", "name": "In Progress" },
  { "id": "31", "name": "Done" },
  { "id": "21", "name": "Blocked" }
]
```

### Execute Transition

```typescript
mcp__plugin_atlassian_atlassian__transitionJiraIssue({
  cloudId: "5d952b27-0223-4b75-8999-6f5dd97440be",
  issueIdOrKey: "ATD-###",
  transition: { id: "11" }  // ID from getTransitionsForJiraIssue
})
```

**Common ATD workflow transitions:**
- `To Do` → `In Progress`
- `In Progress` → `Done`
- `In Progress` → `Blocked`
- `Blocked` → `In Progress`

**Note**: Transition availability depends on current status and workflow configuration. Always fetch available transitions first.

## Add Comments

```typescript
mcp__plugin_atlassian_atlassian__addCommentToJiraIssue({
  cloudId: "5d952b27-0223-4b75-8999-6f5dd97440be",
  issueIdOrKey: "ATD-###",
  body: {
    type: "doc",
    version: 1,
    content: [
      {
        type: "paragraph",
        content: [
          {
            type: "text",
            text: "Comment text here"
          }
        ]
      }
    ]
  }
})
```

**For markdown-style comments**, use the Atlassian Document Format (ADF) structure shown above.

## Examples

### Example 1: Move to In Progress and Assign to Me

User: "Move ATD-836 to In Progress and assign it to me"

1. Load tools: `getTransitionsForJiraIssue`, `transitionJiraIssue`, `editJiraIssue`
2. Get available transitions for ATD-836
3. Find "In Progress" transition ID (e.g., "11")
4. Execute transition:
   ```typescript
   transitionJiraIssue({
     cloudId: "...",
     issueIdOrKey: "ATD-836",
     transition: { id: "11" }
   })
   ```
5. Update assignee:
   ```typescript
   editJiraIssue({
     cloudId: "...",
     issueIdOrKey: "ATD-836",
     update: {
       assignee: [{ set: { accountId: "currentUser" } }]
     }
   })
   ```
6. Confirm: "ATD-836 moved to In Progress and assigned to you"

### Example 2: Mark as Done

User: "Close ATD-837"

1. Load `getTransitionsForJiraIssue`, `transitionJiraIssue`
2. Get transitions for ATD-837
3. Find "Done" transition ID
4. Execute transition
5. Confirm: "ATD-837 marked as Done"

### Example 3: Update Priority to High

User: "Change ATD-838 priority to High"

1. Load `editJiraIssue`
2. Update priority:
   ```typescript
   editJiraIssue({
     cloudId: "...",
     issueIdOrKey: "ATD-838",
     update: {
       priority: [{ set: { name: "High" } }]
     }
   })
   ```
3. Confirm: "ATD-838 priority updated to High"

### Example 4: Add Label

User: "Add the 'Urgent' label to ATD-839"

1. Load `editJiraIssue`
2. Add label:
   ```typescript
   editJiraIssue({
     cloudId: "...",
     issueIdOrKey: "ATD-839",
     update: {
       labels: [{ add: "Urgent" }]
     }
   })
   ```
3. Confirm: "Added 'Urgent' label to ATD-839"

### Example 5: Add Comment

User: "Add a comment to ATD-836 saying 'Fix verified in preview environment'"

1. Load `addCommentToJiraIssue`
2. Add comment with ADF format
3. Confirm: "Comment added to ATD-836"

### Example 6: Unassign Issue

User: "Unassign ATD-840"

1. Load `editJiraIssue`
2. Set assignee to null:
   ```typescript
   editJiraIssue({
     cloudId: "...",
     issueIdOrKey: "ATD-840",
     update: {
       assignee: [{ set: null }]
     }
   })
   ```
3. Confirm: "ATD-840 unassigned"

## Error Handling

### Transition Not Available

If the requested transition isn't available:

```
❌ Cannot move ATD-### to [Status]

Available transitions:
- [List available transitions from getTransitionsForJiraIssue]

Current status: [Current Status]
```

### Invalid Field Value

If field update fails:

```
❌ Cannot update [field] to '[value]'

Error: [Error message from Jira]

Valid values for [field]:
- [List valid options if known]
```

### User Not Found

If assignee lookup fails:

```
❌ User not found: '[query]'

Try:
- Full email address
- Exact display name
- Or use "currentUser" to assign to yourself
```

## Batch Updates

For multiple updates to the same issue:

```
User: "For ATD-836, move it to In Progress, assign to me, and set priority to High"
```

**Approach**: Execute updates sequentially:
1. Transition status
2. Update assignee
3. Update priority

**Confirm all changes**:
```
ATD-836 updated:
✓ Status: In Progress
✓ Assignee: Holden Hewett
✓ Priority: High
```

## Quality Checks

- [ ] Issue key is valid
- [ ] Correct tool selected for update type
- [ ] Transitions fetched before status change
- [ ] Account ID resolved for assignee changes
- [ ] ADF format used for comments
- [ ] All changes confirmed to user
- [ ] Error messages are actionable
