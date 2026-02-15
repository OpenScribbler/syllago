# Write Tests Prompt

Generate comprehensive tests for the following {{language}} code using {{test_framework}}.

## Code to test:
```{{language}}
{{code}}
```

## Test requirements:

1. **Happy path**: Test the expected behavior with valid inputs
2. **Edge cases**: Test boundary conditions (empty inputs, max values, etc.)
3. **Error cases**: Test how the code handles invalid inputs or error conditions
4. **Test naming**: Use descriptive test names that explain what's being tested

## Additional context:
{{additional_context}}

## Output format:
Provide complete, runnable test code with:
- Necessary imports and setup
- Clear test names
- Assertions with helpful failure messages
- Comments explaining non-obvious test cases
