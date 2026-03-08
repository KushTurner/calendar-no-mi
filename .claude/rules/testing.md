---
paths:
  - "**/*_test.go"
  - "**/testdata/**"
---

# Testing

When writing tests, load `ce:writing-tests`.

When fixing flaky tests, load `ce:fixing-flaky-tests`.

| Symptom | Likely Cause |
|---------|--------------|
| Passes alone, fails in suite | Shared state |
| Random timing failures | Race condition |
