# /review Command

Trigger a comprehensive code review of recent changes.

## Usage

```
/review [target]
```

**Arguments**:
- `target` (optional): What to review. Options:
  - `pr` or pull request number - Review a specific PR
  - `branch` - Review all changes in current branch vs main
  - `staged` - Review staged git changes
  - `file:path/to/file` - Review a specific file
  - (default): Review uncommitted changes

## Examples

```
/review
```
Reviews all uncommitted changes in the working directory.

```
/review pr 123
```
Reviews pull request #123.

```
/review branch
```
Reviews all commits in the current branch compared to main.

```
/review file:src/api/auth.py
```
Reviews the specified file.

## What it does

1. Loads the code-review skill and code-reviewer agent
2. Gathers the relevant code changes based on target
3. Performs a systematic review covering:
   - Correctness and logic
   - Security vulnerabilities
   - Performance issues
   - Code readability
   - Test coverage
4. Outputs findings with severity ratings and actionable suggestions

## Output format

```
## Review Summary
- Overall: [Assessment]
- Issues found: X critical, Y high, Z medium, W low

## Detailed Findings

**[Severity]** - `file:line`
[Issue description]
Suggestion: [Specific fix]

## Positive Callouts
- [Something done well]
```
