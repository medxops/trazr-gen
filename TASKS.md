# OpenTelemetry Logs Test Exporter Implementation

Implement a file-based test exporter for OpenTelemetry logs and refactor tests to use it, minimizing changes to current business logic.

## Completed Tasks

- [x] Analyze current test and exporter setup
- [x] Research best practices for test exporters (noop, in-memory, stdout, file)
- [x] Implement a custom in-memory test exporter for logs

## In Progress Tasks

- [ ] Remove the broken stdoutlog/file exporter test from unit tests
- [ ] Ensure the in-memory testLogExporter is the primary unit test for logs
- [ ] Document the in-memory exporter approach in code comments

## Future Tasks

- [ ] For integration tests: use real exporters (file, OTLP) and guard with build tags or skip by default
- [ ] Ensure protocol-driven exporter logic is exercised in integration tests (gRPC, HTTP, file)
- [ ] Add/restore tests for exporter creation and Run logic using dependency injection
- [ ] Document the new test exporter usage in code comments and README

## Implementation Plan

- Remove any unit tests that depend on the stdoutlog/file exporter, as this is not always available and not recommended for unit tests.
- Use the custom in-memory exporter for all log unit tests, allowing assertions on log output without I/O or external dependencies.
- For integration tests, consider using real exporters and guard them with build tags (e.g., //go:build integration) or skip by default.
- Document the rationale for this approach in code comments and in the README.

### Relevant Files

- internal/logs/logs_test.go - Log unit tests, in-memory exporter implementation ✅
- TASKS.md - Task tracking and implementation plan ✅ 