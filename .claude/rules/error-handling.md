---
paths:
  - "**/*.go"
---

# Error Handling

When designing error handling, load `ce:handling-errors`.

- Never swallow errors silently
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Log errors once at the boundary, not at every layer
