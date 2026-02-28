---
description: Apply when the user asks about API design, REST conventions, or endpoint structure
alwaysApply: false
---

# Kitchen Sink Claude Code Rule

This is an example rule that demonstrates description-only activation for Claude Code rules. The AI model decides when to apply it based on the description field.

## API Design Guidelines

When designing or reviewing APIs:

1. **Use nouns for resources** - `/users`, `/orders`, not `/getUsers`, `/createOrder`
2. **HTTP methods for actions** - GET for reads, POST for creates, PUT/PATCH for updates, DELETE for deletes
3. **Consistent naming** - Use camelCase for JSON fields, kebab-case for URL paths
4. **Pagination** - Always paginate list endpoints; use cursor-based pagination for large datasets
5. **Error responses** - Return structured error objects with code, message, and details

## Fields Demonstrated

- **description**: Activation hint for the AI model (triggers on API-related queries)
- **alwaysApply**: Set to false (model-decision activation)
- No globs needed for description-only activation
