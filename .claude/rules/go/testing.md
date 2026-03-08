---
paths:
  - "**/*_test.go"
---

# Go Testing

## Commands

```bash
go test ./...              # Run all tests
go test ./... -v           # Verbose
go test ./... -run TestFoo # Run matching tests
go test -race ./...        # Race detector
```

## Patterns

- Use table-driven tests with `t.Run` for multiple cases
- Mock interfaces by implementing them inline or with a simple struct
- Use `t.Cleanup` instead of `defer` for test teardown
- Prefer `errors.Is` / `errors.As` over string matching in assertions
