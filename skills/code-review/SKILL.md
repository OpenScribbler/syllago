# Code Review Skill

A systematic approach to reviewing code changes for quality, security, and maintainability.

## Review Checklist

### 1. Correctness
- Does the code do what it claims to do?
- Are edge cases handled appropriately?
- Are there any logical errors or off-by-one mistakes?

### 2. Security
- Are user inputs validated and sanitized?
- Are there potential injection vulnerabilities (SQL, XSS, command injection)?
- Are secrets or sensitive data properly protected?
- Are authentication and authorization checks in place?

### 3. Performance
- Are there unnecessary loops or redundant operations?
- Could database queries be optimized (N+1 queries, missing indexes)?
- Are large datasets handled efficiently?
- Is caching used where appropriate?

### 4. Readability & Maintainability
- Are variable and function names clear and descriptive?
- Is the code self-documenting, or does it need comments?
- Is the logic easy to follow without mental gymnastics?
- Are functions/methods focused on a single responsibility?

### 5. Testing
- Are there tests covering the new functionality?
- Do tests cover both happy paths and error cases?
- Are tests readable and maintainable?

### 6. Code Style
- Does the code follow the project's style guide?
- Is formatting consistent with the rest of the codebase?

## Output Format

For each issue found, provide:
- **Severity**: Critical, High, Medium, Low
- **Location**: File and line number
- **Issue**: Clear description of the problem
- **Suggestion**: Specific recommendation for fixing it

## Example Output

```
**Medium** - `src/api/users.py:45`
User input is not validated before database query. This could allow malformed data to cause errors.
Suggestion: Add input validation using Pydantic model or manual checks before the query.
```
