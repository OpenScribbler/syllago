# /astro - Intelligent Astro Agent Router

Intelligent routing system for Astro/Starlight development tasks with autonomous validation loops and retry logic.

## Purpose

Route user requests to appropriate specialized worker agents (component, UI, debug, fix, refactor, a11y), automatically validate their output, retry with feedback on failures, and generate comprehensive reports with full reasoning capture.

## Usage

```bash
# Basic usage (interactive mode)
/astro create a tooltip component for showing definitions

# Auto mode (skip confirmations)
/astro --auto fix spacing in Definition.astro

# Custom retry limit
/astro --test-retries 5 update Definition component to show hover text
```

## Flags

- `--auto`: Skip confirmations between agent invocations (default: interactive)
- `--test-retries N`: Maximum retry attempts (default: 3, max: 10)

## Workflow

### Phase 1: Session Initialization

1. **Generate Session ID**
   ```javascript
   const sessionId = crypto.randomUUID(); // e.g., "550e8400-e29b-41d4-a009-426655440000"
   ```

2. **Create Log File**
   - Path: `(deprecated: .agent-os/logs/){YYYY-MM-DD}/{sessionId}-{concise-summary}.jsonl`
   - Format: JSONL (JSON Lines) - one JSON object per line
   - Detail Level: Debug (full verbosity)

3. **Log Session Start**
   ```jsonl
   {"timestamp":"2025-10-08T14:30:00Z","event":"session_start","session_id":"{sessionId}","user_input":"{original request}","flags":{"auto":false,"test_retries":3}}
   ```

### Phase 2: Intelligent Routing

4. **Parse User Input**
   - Extract task type, target component/file, requirements
   - Identify keywords matching agent specializations
   - Consider context and ambiguity

5. **Select Worker Agent**

   Use this routing table:

   | Task Type | Keywords/Patterns | Agent | Example |
   |-----------|-------------------|-------|---------|
   | **New Component** | "create", "new", "build", "add component" | astro-component | "create a tooltip component" |
   | **Component Enhancement** | "update", "add feature to", "enhance", "modify [component] to" | astro-component | "update Definition to show display text" |
   | **Styling/Visual** | "fix spacing", "adjust colors", "change layout", "CSS" | astro-ui | "fix spacing in Definition" |
   | **Bug Fix** | "broken", "not working", "error in behavior", "fix [behavior]" | astro-fix | "tooltip doesn't close on click" |
   | **Build/Runtime Error** | "build fails", "TypeScript error", "runtime error" | astro-debug | "TypeScript error in Definition.astro" |
   | **Code Quality** | "refactor", "improve performance", "clean up", "optimize" | astro-refactor | "refactor Definition to use composition" |
   | **Accessibility** | "a11y", "accessibility", "WCAG", "keyboard", "screen reader" | astro-a11y | "add ARIA labels to Definition" |

   **Disambiguation Logic**:

   **Scenario 1: Missing Target**
   - User: "/astro fix the spacing"
   - Router asks: "Which component or file has spacing issues?"
   - Log: `{"event":"clarification_requested","reason":"Missing target component"}`

   **Scenario 2: Multiple Concerns**
   - User: "/astro the tooltip is blue and doesn't close"
   - Option A: Ask "Should I prioritize the styling (blue color) or the behavior (not closing)?"
   - Option B: Route to primary concern (astro-fix for behavior), validator catches both issues
   - Log: `{"event":"main_claude_routing","detected_agents":["astro-fix","astro-ui"],"primary":"astro-fix","strategy":"Primary behavior fix, UI will be validated"}`

   **Scenario 3: Multi-Agent Detection**
   - User: "/astro create accessible tooltip component"
   - Router decision: Route to astro-component (primary), validator automatically runs a11y tests
   - Log: `{"event":"main_claude_routing","detected_agents":["astro-component","astro-a11y"],"primary":"astro-component","strategy":"Component creation with a11y validation"}`

   **Default Behavior**:
   - If unclear after analysis → Default to astro-component (most general)
   - If no keywords match → Ask clarifying question
   - Always log reasoning for routing decision

6. **Log Routing Decision**
   ```jsonl
   {"timestamp":"2025-10-08T14:30:05Z","event":"main_claude_routing","session_id":"{sessionId}","reasoning":"User reported spacing issues in Definition component with punctuation. This is a visual/UI problem, not a build error or logic bug. Selecting astro-ui agent as it specializes in visual fixes and will apply visual verification protocol.","detected_agents":["astro-ui"],"user_input":"{original request}"}
   ```

### Phase 3: Context Gathering

7. **Invoke Context-Fetcher Agent**
   - Extract file paths from request
   - Identify target pages for testing
   - Collect any requirements or specifications
   - Read relevant files if needed

8. **Log Context Gathering**
   ```jsonl
   {"timestamp":"2025-10-08T14:30:10Z","event":"context_gathered","session_id":"{sessionId}","context":{"target_file":"aembit-docs/src/components/Definition.astro","target_page":"/docs/trust-providers","expected_outcome":"No extra spacing before punctuation"}}
   ```

### Phase 4: Worker Agent Invocation

9. **Invoke Worker Agent**
   - Use Task tool with appropriate agent type
   - Provide all required context variables
   - Set clear success criteria

10. **Log Agent Invocation**
    ```jsonl
    {"timestamp":"2025-10-08T14:30:15Z","event":"agent_invoked","session_id":"{sessionId}","agent":"astro-ui","attempt":1,"context_provided":{"TASK_DESCRIPTION":"Fix spacing issues...","TARGET_FILE":"...","TARGET_PAGE":"...","EXPECTED_OUTCOME":"..."}}
    ```

11. **Wait for Agent Completion**

12. **Log Agent Completion**
    ```jsonl
    {"timestamp":"2025-10-08T14:30:45Z","event":"agent_complete","session_id":"{sessionId}","agent":"astro-ui","attempt":1,"reasoning":"Problem Analysis: Definition component uses inline-block display which creates whitespace nodes between elements, causing extra spacing and unwanted line breaks before punctuation. Implementation: Changed display from inline-block to inline-flex to eliminate whitespace while maintaining inline flow. Also adjusted margin-right to 0.1em to prevent punctuation collision. Why This Approach: Inline-flex is the modern solution that eliminates whitespace issues without HTML restructuring. Alternatives Considered: font-size:0 hack (rejected - fragile), removing HTML whitespace (rejected - hard to maintain). Proof: BEFORE/AFTER screenshots show spacing corrected, console clean with 0 errors.","files_modified":["aembit-docs/src/components/Definition.astro"],"report_path":"(deprecated: .agent-os/reports/)astro/2025-10-08-14-30-45_astro-ui_Definition-report.md"}
    ```

### Phase 5: Validation Loop

13. **Invoke Validator**
    - Use Task tool to invoke astro-validator agent
    - Provide worker report, task context, target files
    - Include attempt number

14. **Log Validation Start**
    ```jsonl
    {"timestamp":"2025-10-08T14:30:50Z","event":"validation_start","session_id":"{sessionId}","validator":"astro-validator","worker_validated":"astro-ui","attempt":1}
    ```

15. **Validator Executes Tests**
    - Validator selects appropriate tests
    - Populates variables
    - Executes procedures
    - Interprets results

16. **Validator Returns Result**
    - Status: PASS or FAIL
    - Tests run with results
    - Reasoning for determination
    - Feedback (if FAIL)

17. **Log Validation Complete**

    **If PASS**:
    ```jsonl
    {"timestamp":"2025-10-08T14:31:15Z","event":"validation_complete","session_id":"{sessionId}","validator":"astro-validator","worker_validated":"astro-ui","attempt":1,"status":"PASS","tests_run":[{"test":"ui/visual-verification","status":"passed"},{"test":"common/console-clean","status":"passed"},{"test":"common/typescript-check","status":"passed"}],"reasoning":"All tests passed. Visual verification shows spacing fixed, console has 0 errors, TypeScript validation clean."}
    ```

    **If FAIL**:
    ```jsonl
    {"timestamp":"2025-10-08T14:30:56Z","event":"validation_failed","session_id":"{sessionId}","validator":"astro-validator","worker_validated":"astro-ui","attempt":1,"reasoning":"Test Selection: Ran ui/visual-verification and common/console-clean tests because astro-ui made CSS changes affecting visual appearance. Test Interpretation: Visual verification passed - screenshots confirm spacing fixed. However, console-clean test failed with 2 TypeErrors. Why Failed: Runtime errors detected during dev server testing. The component references 'term' property that doesn't exist in Props interface. This will cause production failures even though visual appearance is correct. Feedback Rationale: astro-ui must add 'term' to Props interface before validation can pass. Visual fix is good, but runtime stability required.","tests_run":[{"test":"ui/visual-verification","status":"passed"},{"test":"common/console-clean","status":"failed","errors":["TypeError: undefined property 'term' at Definition.astro:45"]}],"feedback":"Add 'term: string' to Props interface in Definition.astro. The component references props.term but this property is not defined in the interface, causing runtime TypeError."}
    ```

### Phase 6: Retry Logic (if FAIL)

18. **Decide on Retry**
    - Check current attempt vs max retries
    - If under limit → Retry
    - If at limit → Escalate to human

19. **Log Retry Decision**
    ```jsonl
    {"timestamp":"2025-10-08T14:31:00Z","event":"retry_decision","session_id":"{sessionId}","attempt":1,"max_retries":3,"decision":"RETRY","reasoning":"Validation failed with fixable issues. Attempt 1 of 3. Providing specific feedback to worker: 'Add term property to Props interface'. This is a simple fix that should resolve the issue."}
    ```

20. **Re-invoke Worker with Feedback**
    - Include validator feedback in context
    - Increment attempt number
    - Return to Phase 4

21. **If Max Retries Reached**

    **Log Escalation**:
    ```jsonl
    {"timestamp":"2025-10-08T14:35:00Z","event":"escalation","session_id":"{sessionId}","attempt":3,"max_retries":3,"reasoning":"Validation failed after 3 attempts. Issues persist: TypeScript errors still present. Escalating to human for manual intervention. The worker may need different approach or the validation criteria may need adjustment.","persistent_issues":["TypeScript errors in Props interface","Console runtime errors"]}
    ```

### Phase 7: Success & Report Generation

22. **Generate Combined Report**
    - Summarize all worker attempts
    - Include all validation results
    - Show retry history
    - Link to detailed reports and logs

23. **Save Combined Report**
    - Path: `(deprecated: .agent-os/reports/)astro/{YYYY-MM-DD}_{concise-summary}.md`

24. **Log Session End**
    ```jsonl
    {"timestamp":"2025-10-08T14:31:20Z","event":"session_end","session_id":"{sessionId}","status":"SUCCESS","total_attempts":1,"worker_agent":"astro-ui","validation_result":"PASS","reports":{"worker":"(deprecated: .agent-os/reports/)astro/2025-10-08-14-30-45_astro-ui_Definition-report.md","validator":"(deprecated: .agent-os/reports/)astro/2025-10-08-14-31-15_astro-validator_astro-ui-report.md","combined":"(deprecated: .agent-os/reports/)astro/2025-10-08_fix-Definition-spacing.md"},"log_file":"(deprecated: .agent-os/logs/)2025-10-08/550e8400-e29b-41d4-a009-426655440000-fix-Definition-spacing.jsonl"}
    ```

25. **Present Summary to User**
    ```markdown
    ✅ Task completed successfully!

    **Agent**: astro-ui
    **Attempts**: 1
    **Validation**: PASSED

    **Changes**: Fixed spacing in Definition component by changing display from inline-block to inline-flex

    **Reports**:
    - Worker: (deprecated: .agent-os/reports/)astro/2025-10-08-14-30-45_astro-ui_Definition-report.md
    - Validator: (deprecated: .agent-os/reports/)astro/2025-10-08-14-31-15_astro-validator_astro-ui-report.md
    - Combined: (deprecated: .agent-os/reports/)astro/2025-10-08_fix-Definition-spacing.md

    **Session Log**: (deprecated: .agent-os/logs/)2025-10-08/550e8400-e29b-41d4-a009-426655440000-fix-Definition-spacing.jsonl
    ```

## JSONL Logging Schema

```typescript
interface LogEntry {
  timestamp: string;        // ISO 8601: "2025-10-08T14:30:00Z"
  session_id: string;       // UUID v4
  event: LogEvent;          // Event type
  agent?: string;           // Agent name (if applicable)
  attempt?: number;         // Retry attempt (1-indexed)
  reasoning?: string;       // Narrative reasoning
  [key: string]: unknown;   // Event-specific fields
}

type LogEvent =
  | "session_start"
  | "main_claude_routing"
  | "context_gathered"
  | "agent_invoked"
  | "agent_complete"
  | "validation_start"
  | "validation_complete"
  | "validation_failed"
  | "retry_decision"
  | "escalation"
  | "session_end";
```

## Reasoning Capture Requirements

Every log entry with a `reasoning` field MUST include:

**For Routing**:
- Why this agent was selected
- What keywords/patterns matched
- Alternative agents considered

**For Agent Completion**:
- Problem analysis (what was the issue)
- Implementation decisions (what was done)
- Rationale (why this approach)
- Alternatives considered (what was rejected)
- Proof of quality (test results, screenshots)

**For Validation**:
- Test selection rationale
- Variable population sources
- Result interpretation
- Pass/Fail determination reasoning
- Feedback rationale (if failed)

**For Retry Decisions**:
- Why retry is appropriate
- What feedback will help
- Confidence in resolution

**For Escalation**:
- Why max retries reached
- What issues persist
- Why human intervention needed

## MVP Agent Roster (Tasks 3-6)

- **astro-component**: Component creation and enhancement
- **astro-ui**: Visual and styling fixes

## Full System Agent Roster (Tasks 7+)

- **astro-component**: Component creation and enhancement
- **astro-ui**: Visual and styling fixes
- **astro-debug**: Build/runtime/TypeScript error resolution
- **astro-fix**: Logic and behavior bug fixes
- **astro-refactor**: Code quality and performance improvements
- **astro-a11y**: Accessibility compliance (WCAG 2.1 AA)

## Error Handling

- **Invalid agent**: Ask for clarification
- **Missing context**: Request required information
- **Worker failure**: Check max retries, retry with feedback or escalate
- **Validator failure**: Log error, escalate to human
- **Log write failure**: Continue but warn user

## Examples

**Example 1: Component Enhancement**
```bash
/astro update Definition component to show displayText in tooltip when provided
```
→ Routes to astro-component → Creates/enhances Props interface → Validates → Reports

**Example 2: UI Fix**
```bash
/astro --auto fix spacing in Definition.astro before punctuation
```
→ Routes to astro-ui → Captures before/after screenshots → Validates console + visual → Reports

**Example 3: Multi-Retry Scenario**
```bash
/astro create accessible navigation menu
```
→ Routes to astro-component → Validator catches missing ARIA → Retry with feedback → Validator catches keyboard nav → Retry → PASS

**Example 4: Escalation**
```bash
/astro refactor Database component to use new pattern
```
→ Routes to astro-refactor → Fails validation (build errors) → Retry → Fails again → Retry → Fails third time → ESCALATE

## Implementation Notes

- Use Task tool to invoke worker agents
- Use Task tool to invoke astro-validator
- Generate UUIDs for session IDs
- Ensure JSONL is properly formatted (one JSON object per line, no trailing commas)
- Create log directory structure if needed
- Handle concurrent sessions (unique session IDs prevent conflicts)
