---
paths:
  - "**/*"
---

# Verification

Before claiming work is complete, load `ce:verification-before-completion`.

```bash
go build ./...   # Must pass
go test ./...    # Must pass
go vet ./...     # Must pass
```
