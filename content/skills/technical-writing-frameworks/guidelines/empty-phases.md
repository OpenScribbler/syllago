# Handling Empty Phases and Sections

**Rule**: Never leave a section empty or implied. If no issues found, explicitly say so. Silence creates ambiguity -- did you analyze and find nothing, or forget to analyze?

## Templates by Section Type

### Empty Priority Level
```
## [Priority Level]
**No [level] issues identified.** [Optional 1-sentence context]
```

### Empty Framework Analysis
```
## [Framework] Analysis
**Assessment: No framework violations identified.** [1-2 sentences on what was analyzed and why it's aligned]
```

### Empty Trade-offs
```
## Trade-offs
**No conflicting persona needs requiring trade-off decisions.** All feedback is aligned and mutually reinforcing.
```

### Empty Fact-Checking / Browser Review
```
**No [fact-checking/browser review] required.** [Reason: only Aembit content / no visual issues flagged / etc.]
```

### Document Splitting Not Needed
```
**Not applicable.** Document has clear, focused purpose serving single audience with single action.
```

## When to Add Context

**Add context when**: Empty section might surprise the user (e.g., "No P0 despite 4 personas flagging concerns -- all issues are P1/P2 severity"), empty section is notable ("All 5 personas aligned -- unusual"), or empty section might indicate incomplete analysis ("No Troubleshoot content found, but appropriate for Tutorial type").

**Skip context when**: Empty section is expected and unremarkable (e.g., "No P3 suggestions"), or context would be redundant with other sections.

## Rules

- Minimum statement: "No [X] identified."
- With context (when helpful): "No [X] identified. [Brief explanation]"
- Never: Leave section empty, use "TBD" or "None", use unclear language like "Nothing here"
