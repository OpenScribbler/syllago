---
description: Comments explain why, not what
alwaysApply: true
---

Write comments that explain **why**, not what. Good identifiers already say what the code does.

Add a comment only when:
- The reason for an approach is non-obvious or counter-intuitive
- A constraint or workaround justifies unusual-looking code
- A subtle invariant would surprise a future reader

Never add comments that describe what the code does (e.g., `// increment counter` above `i++`). If code needs a comment to explain what it does, rename the identifier or extract a function instead.
