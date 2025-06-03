# Project Architecture: TRAZR-GEN

This document describes the high-level architecture, key packages, and design principles of the `trazr-gen` project.

---

## Overview

`TRAZR-GEN` is a modular, idiomatic Go CLI application designed for observability, metrics, and trace data generation, following Clean Architecture and modern Go best practices for maintainability, testability, and scalability. For regulated environments like healthcare, it includes robust features for handling sensitive data. This allows users to define schemas for generating realistic, synthetic mock data, mirroring production environments without exposing confidential details. It also supports masking or anonymizing sensitive fields in existing datasets. When generating traces or metrics from live systems, `TRAZR-GEN` can be configured to redact or filter out sensitive information using pattern matching or policy-driven redaction, ensuring data privacy and regulatory compliance.

---

## Directory Structure

```
├── cmd/                # Application entrypoints (main.go for each CLI)
│   └── trazr-gen/      # Main CLI command and config
├── internal/           # Internal helpers and test logic (not exposed externally)
│   ├── common/         # Shared helpers: config, attributes, logging, validation, mock data
│   └── e2etest/        # End-to-end and integration tests
├── pkg/                # Core application logic (metrics, logs, traces)
│   ├── metrics/        # Metrics generation logic
│   ├── logs/           # Log signal generation logic
│   └── traces/         # Trace signal generation logic
├── build/              # Build artifacts
├── docs/               # Documentation and logo
├── .github/            # GitHub Actions workflows, issue/PR templates
├── Dockerfile, Makefile, .golangci.yml, etc.
```

---

## Key Packages and Responsibilities

- **cmd/**: Entrypoints for CLI applications. Contains `main.go` and config for the CLI.
- **internal/common/**: Shared helpers for config, attributes, logging, validation, and mock data.
- **internal/e2etest/**: End-to-end and integration tests.
- **pkg/metrics/**: Business logic for generating synthetic OpenTelemetry metrics (histogram, gauge, sum, etc.).
- **pkg/logs/**: Business logic for generating synthetic OpenTelemetry logs.
- **pkg/traces/**: Business logic for generating synthetic traces and scenarios.

---

## Design Principles

- **Clean Architecture**: Separation of concerns between CLI, business logic, and data layers.
- **Domain-Driven Design**: Code is grouped by feature/domain for clarity and cohesion.
- **Interface-Driven Development**: All public functions interact with interfaces, not concrete types, for testability and flexibility.
- **Dependency Injection**: Dependencies are injected via constructors, avoiding global state.
- **Observability**: OpenTelemetry is used for tracing, metrics, and logging. Context propagation is enforced throughout.
- **Testing**: Table-driven and parallelizable unit tests. Integration tests are separated using build tags.

---

## Observability & Instrumentation

- All metrics and traces are generated using OpenTelemetry APIs.
- Context (`context.Context`) is propagated through all major functions.
- Logging is structured and correlates with trace IDs.

---

## Extending the Project

- **Add a new CLI command**: Create a new file in `cmd/trazr-gen/` and register it in `main.go`.
- **Add a new metric, log, or trace scenario**: Implement in `pkg/metrics/`, `pkg/logs/`, or `pkg/traces/`, and expose via CLI.
- **Add integration tests**: Place in `internal/e2etest/` or use build tags for separation.

---

## References
- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [Clean Architecture](https://8thlight.com/blog/uncle-bob/2012/08/13/the-clean-architecture.html)
- [OpenTelemetry for Go](https://opentelemetry.io/docs/instrumentation/go/)

---

For more details, see the `README.md` or open an issue/discussion.

---

## CI/CD and Security Controls

This project is designed for robust, secure, and compliant software delivery, suitable for healthcare and regulated environments.

### Build and Test Pipeline
- All code is built and tested in a dedicated `build-and-test` job before any release is created.
- Go modules and the build cache are used for efficient builds.
- Linting and static analysis are enforced via GitHub Actions.

### Release Process
- Releases are triggered by pushing a Git tag or via manual workflow dispatch.
- The release job only runs if all tests pass.
- GoReleaser builds binaries for all supported platforms (Linux, macOS, Windows, ARM, AMD64, etc.).
- All actions in the workflow are pinned to full commit SHAs to ensure supply chain security.
- Explicit permissions are set for each job, following the principle of least privilege.

### Security and Compliance
- **SBOM Generation:** Every release includes a Software Bill of Materials (SBOM) generated with Syft, uploaded as both a workflow artifact and to GitHub's SBOM API.
- **Snyk Scanning:** Snyk scans are run on every PR and push to main, with SARIF results uploaded to GitHub Code Scanning.
- **GPG Key Handling:** GPG keys are imported, trusted, and cleaned up securely in CI/CD workflows. All artifacts are signed.
- **Secret Management:** All sensitive secrets (GPG, Snyk, Docker, etc.) are stored in GitHub Secrets and rotated regularly.
- **Branch Protection:** Branch protection rules and required status checks are enforced on main.
- **Dependabot and Secret Scanning:** Dependabot alerts, security updates, and secret scanning are enabled.
- **Audit Logging:** Organization-level audit logging is enabled for compliance and incident response.
- **Third-Party Actions:** Only trusted, pinned actions are used in workflows.

### Artifact and Release Management
- All build artifacts, SBOMs, and security reports are uploaded as workflow artifacts for traceability and auditing.
- Homebrew formulae and Docker images are published as part of the release process.

### Documentation and Review
- This architecture and security posture are documented and reviewed regularly.
- For more details, see `SECURITY.md` and `CONTRIBUTING.md`.

---

For questions about the architecture or security controls, contact eric.stpierre@medoya.com. 