# OpenTelemetry Logs Test Exporter Implementation

Implement a file-based test exporter for OpenTelemetry logs and refactor tests to use it, minimizing changes to current business logic.

## Completed Tasks

- [x] Analyze current test and exporter setup
- [x] Research best practices for test exporters (noop, stdout, file)

## In Progress Tasks

- [ ] Implement a file-based test exporter for logs
- [ ] Refactor `internal/logs/logs_test.go` to use the file-based exporter for integration-style tests
- [ ] Ensure protocol-driven exporter logic is exercised in tests (gRPC, HTTP, file)
- [ ] Add/restore tests for exporter creation and `Run` logic using dependency injection
- [ ] Document the new test exporter usage in code comments

## Future Tasks

- [ ] Add integration tests with a local OpenTelemetry Collector (optional)
- [ ] Add coverage for error handling and edge cases in exporter logic

## Implementation Plan

- Create a simple file-based exporter for logs in the test code (or as a test utility)
- Use dependency injection to pass the test exporter to the code under test
- Refactor or re-enable previously skipped/commented tests to use the new exporter
- Minimize changes to business logic; only touch test code and injection points
- Document the approach in code comments and in this task list

### Relevant Files

- internal/logs/logs_test.go - Main log tests, to be refactored to use the file exporter
- internal/logs/logs.go - Only update if needed for dependency injection
- TASKS.md - This task list and implementation plan 