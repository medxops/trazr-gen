<!-- Project Logo -->
<p align="center">
  <img src="docs/logo.png" alt="trazr-gen logo" width="800"/>
</p>

<p align="center">
  <a href="https://github.com/medxops/trazr-gen/actions/workflows/test.yaml"><img src="https://github.com/medxops/trazr-gen/actions/workflows/test.yaml/badge.svg" alt="Build Status"></a>
  <a href="https://pkg.go.dev/github.com/medxops/trazr-gen"><img src="https://pkg.go.dev/badge/github.com/medxops/trazr-gen.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/medxops/trazr-gen"><img src="https://goreportcard.com/badge/github.com/medxops/trazr-gen" alt="Go Report Card"></a>
  <a href="https://codecov.io/gh/medxops/trazr-gen"><img src="https://codecov.io/gh/medxops/trazr-gen/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License: Apache-2.0"></a>
</p>

---

A modular Go CLI application for generating OpenTelemetry logs, metrics, and traces with enhanced trace generation including metrics with exemplars for correlation and debugging. Designed for maintainability, scalability, and robust handling of sensitive data—ideal for regulated environments (healthcare, finance, etc.).

---


## Features

- Generate OpenTelemetry **logs**, **metrics**, and **traces**
- **Enhanced trace generation** with realistic patterns (simple, chain, fan-out, complex, random)
- **Metrics with exemplars** linking to trace and span IDs for correlation and debugging
- **Error control presets** for clean, stress, and chaos testing scenarios
- **Reproducible generation** with configurable random seeds for consistent test data
- **Flexible CLI and YAML** config file support
- Mock data generation with [gofakeit](https://github.com/brianvoe/gofakeit)
- **Sensitive data flagging** for attributes and headers
- **Human-friendly** terminal output and machine-readable JSON logs
- **Unified output control** with consistent `--print-details` flag across all signals
- Distributed tracing and metrics with OpenTelemetry
- **Seed tracking** in resource attributes for debugging and reproducibility

---

## Installation

### macOS/Linux

Install with Homebrew:
```sh
brew tap medxops/toolkit
brew install trazr-gen
```

You can generate signals right away if you have a collector setup at `localhost:4318`:
```sh
trazr-gen logs --logs 10
```

### Docker

```sh
docker pull ghcr.io/medxops/trazr-gen:latest
```
Same example as above:
```sh
docker run --rm ghcr.io/medxops/trazr-gen:latest logs --logs 10 --otlp-endpoint host.docker.internal:4318
```
> **Note:** Use `host.docker.internal` for the OTLP endpoint to connect to a collector running on your host (macOS/Windows). On Linux, use `--network=host` and `localhost`.

### Binaries for Windows or other platforms 

- **Download the latest prebuilt binary** from the [Releases page](https://github.com/medxops/trazr-gen/releases).
  - Choose the appropriate `.tar.gz` or `.zip` file for your platform and architecture (e.g., `trazr-gen_windows_amd64.tar.gz`).
  - Unpack it (using 7-Zip, WinRAR, or built-in tools), then run `trazr-gen.exe` from the extracted folder:
    ```sh
    .\trazr-gen.exe logs --logs 10
    ```

### Go (cross-platform, requires Go 1.20+)

- **Install with Go** (requires Go 1.20+):
  ```sh
  go install github.com/medxops/trazr-gen@latest
  ```

- **Build from source:**
  ```sh
  git clone https://github.com/medxops/trazr-gen.git
  cd trazr-gen
  go build -o trazr-gen.exe ./cmd/trazr-gen
  ```

---

## Run Examples

Generate 10 logs with mock data (collector at http://localhost:4318):
```sh
trazr-gen logs --logs 10
```

Generate metrics for 5 seconds:
```sh
trazr-gen metrics --duration 5s
```

Generate traces with 3 child spans:
```sh
trazr-gen traces --child-spans 3
```

**Enhanced trace generation with logs and metrics:**
```sh
trazr-gen traces --enhanced --spans-per-trace 10 --pattern chain --error-rate 0.1
```

**Reproducible generation with fixed seed:**
```sh
trazr-gen traces --mock-seed 12345 --enhanced --spans-per-trace 5
```

---

## Configuration

TRAZR-GEN supports configuration via CLI flags or a YAML config file. See all options in [config.yaml](https://github.com/medxops/trazr-gen/blob/main/config.yaml).

---

## CLI Usage

```sh
trazr-gen [command] [flags]
```

### Commands
- `logs`    Generate OpenTelemetry logs
- `metrics` Generate OpenTelemetry metrics
- `traces`  Generate OpenTelemetry traces

### Common Flags
- `--config`           Path to config file
- `--mock-data`        Enable mock data templates
- `--mock-seed`        Random seed for reproducible generation (0 = timestamp-based, any other value = fixed seed)
- `--otlp-endpoint`    OTLP exporter endpoint
- `--service`          Service name
- `--log-level`        Log level (debug, info, warn, error)
- `--terminal-output`  Enable/disable terminal output instead of json log
- `--print-details`    Print detailed record output to terminal for each item (default: true)
- `--show-progress-counter` Show progress counter (X generated: Y) for each item (default: true)

### Output Control

TRAZR-GEN provides flexible output control for different use cases:

#### `--print-details` Flag
The `--print-details` flag works consistently across all three signals (logs, metrics, and traces):

- **`--print-details`** (default: `true`) - Shows both detailed records AND progress counter
- **`--print-details=false`** - Shows only progress counter (like "Logs generated: X")

**Examples:**
```sh
# Default behavior: detailed output + progress counter
trazr-gen logs --logs 5 --terminal-output

# Only progress counter (no detailed output)
trazr-gen logs --logs 5 --terminal-output --print-details=false

# Same unified flag works for all signals
trazr-gen metrics --metrics 5 --terminal-output --print-details=false
trazr-gen traces --traces 5 --terminal-output --print-details=false
```

#### `--show-progress-counter` Flag
The `--show-progress-counter` flag controls the progress tracking display:

- **`--show-progress-counter`** (default: `true`) - Shows "X generated: Y" for each item
- **`--show-progress-counter=false`** - Hides progress counter, shows only detailed output

**Combined Usage:**
```sh
# Both detailed output and progress counter (default)
trazr-gen logs --logs 5 --terminal-output

# Only detailed output (no progress counter)
trazr-gen logs --logs 5 --terminal-output --show-progress-counter=false

# Only progress counter (no detailed output)
trazr-gen logs --logs 5 --terminal-output --print-details=false

# No output at all (silent mode)
trazr-gen logs --logs 5 --terminal-output --print-details=false --show-progress-counter=false
```

### Enhanced Trace Generation Flags
- `--enhanced`         Enable enhanced trace generation with logs, metrics, and exemplars
- `--clean`            Generate clean traces with no errors and minimal warnings
- `--stress`           Generate stress traces with high error rate (20%) for testing error handling
- `--chaos`            Generate chaos traces with very high error rate (50%) for chaos testing
- `--error-rate`       Probability of span failure (0.0 to 1.0)
- `--spans-per-trace`  Number of spans per trace (overrides child-spans)
- `--pattern`          Trace structure pattern: simple, chain, fan-out, complex, random
- `--services`         Comma-separated list of service names

See `trazr-gen [command] --help` or [config.yaml](https://github.com/medxops/trazr-gen/blob/main/config.yaml) for all options.

---

## Enhanced Trace Generation

TRAZR-GEN supports **enhanced trace generation** with realistic patterns and integrated logs/metrics:

### Features

- **Realistic trace patterns**: simple, chain, fan-out, complex, random
- **Multi-service traces** with configurable service names
- **Span events** with contextual information
- **Rich attributes** with service-specific metadata
- **Error simulation** with configurable error rates
- **Error control presets** for different testing scenarios
- **Automatic logs and metrics** (1-3 per span) with service-appropriate content
- **Metrics with exemplars** linking to trace and span IDs for correlation and debugging

### Error Control

TRAZR-GEN provides flexible error control for different testing scenarios:

#### Preset Modes
- **`--clean`**: Generate clean traces with no errors and minimal warnings
- **`--stress`**: Generate stress traces with high error rate (20%) for testing error handling
- **`--chaos`**: Generate chaos traces with very high error rate (50%) for chaos testing

#### Custom Error Rates
- **`--error-rate <value>`**: Set custom error probability (0.0 to 1.0)

### Metrics with Exemplars

Enhanced trace generation automatically creates **metrics with exemplars** that link to trace and span IDs. Exemplars provide correlation between metrics and traces, enabling powerful debugging and analysis:

- **Trace Correlation**: Each metric includes the associated trace ID
- **Span Correlation**: Each metric includes the associated span ID  
- **Realistic Values**: Metrics represent span performance characteristics
- **Rich Attributes**: Include service names, operation types, and status codes
- **OpenTelemetry Compliant**: Follows official exemplar specifications
- **Automatic Generation**: 1-3 metrics per span with deterministic patterns

**Example metric with exemplar:**
```
Generated Metric Record:
  Name: span.duration
  Type: Gauge
  Value: 0
  Time: 2025-07-29T21:32:14.81938-04:00
  Attributes:
    - error: true
    - operation: create_user
    - service: user_service
    - span.name: process_request
    - status: success
  TraceID: 1bb784680432b18c43ab000ad1c98e85
  SpanID: 0628a68fec6022cc
  Resource Attributes:
    - service.name: trazr-gen
    - trazrgen.random.seed: timestamp-based
```

**Metric Types Generated:**
- `span.duration` - Performance metrics
- `span.events` - Event count metrics  
- `span.errors` - Error rate metrics
- `span.attributes` - Attribute count metrics
- `span.attributes.count` - Detailed attribute metrics

### Exemplar Generation

Exemplars are automatically generated when using enhanced trace generation. Each span generates 1-3 metrics with exemplars, providing rich correlation data:

**How Exemplars Work:**
1. **Automatic Generation**: Every span in enhanced mode generates metrics with exemplars
2. **Trace Correlation**: Each metric includes the trace ID for linking back to the trace
3. **Span Correlation**: Each metric includes the span ID for linking to specific spans
4. **Deterministic Patterns**: With `--mock-seed`, exemplar generation is reproducible
5. **Rich Context**: Attributes include service, operation, status, and error information

**Exemplar Use Cases:**
- **Performance Analysis**: Correlate metrics with specific traces and spans
- **Error Investigation**: Link error metrics to failed spans for debugging
- **Service Monitoring**: Track metrics by service and operation type
- **Trace Correlation**: Use exemplars to navigate from metrics to traces in observability tools

**Example Commands for Exemplar Generation:**
```bash
# Basic exemplar generation (1-3 metrics per span)
./build/trazr-gen traces --enhanced --spans-per-trace 3

# High-volume exemplar generation
./build/trazr-gen traces --enhanced --spans-per-trace 20 --pattern complex

# Service-specific exemplars
./build/trazr-gen traces --enhanced --services "api-gateway,user-service,payment-service" --spans-per-trace 8

# Reproducible exemplars for testing
./build/trazr-gen traces --enhanced --mock-seed 12345 --spans-per-trace 5

# Error-focused exemplars
./build/trazr-gen traces --enhanced --stress --spans-per-trace 10
```

### Examples

```sh
# Generate clean traces (no errors)
trazr-gen traces --enhanced --clean

# Generate stress traces for error handling testing
trazr-gen traces --enhanced --stress

# Generate chaos traces for chaos testing
trazr-gen traces --enhanced --chaos

# Generate traces with custom error rate
trazr-gen traces --enhanced --error-rate 0.1

# Generate enhanced traces with logs and metrics (including exemplars)
trazr-gen traces --enhanced --spans-per-trace 10 --pattern complex

# Simple enhanced generation (includes exemplars)
trazr-gen traces --enhanced

# Enhanced with reproducible seed (deterministic exemplars)
trazr-gen traces --enhanced --mock-seed 12345

# Generate exemplars with specific services
trazr-gen traces --enhanced --services "web-service,database-service,cache-service" --spans-per-trace 5

# Clean traces with exemplars (no errors)
trazr-gen traces --enhanced --clean --spans-per-trace 3

# Stress testing with exemplars (high error rate)
trazr-gen traces --enhanced --stress --spans-per-trace 8
```

---

## Error Control

TRAZR-GEN provides comprehensive error control for different testing and monitoring scenarios:

### Use Cases

- **Clean Testing**: Generate error-free traces for baseline performance testing
- **Error Handling Testing**: Test your monitoring and alerting systems with controlled error rates
- **Chaos Testing**: Simulate high-failure scenarios to test system resilience
- **Custom Scenarios**: Fine-tune error rates for specific testing needs

### Error Control Modes

#### Clean Mode (`--clean`)
- **Error Rate**: 0% (no span errors)
- **Use Case**: Baseline testing, performance validation
- **Example**: `trazr-gen traces --enhanced --clean`

#### Stress Mode (`--stress`)
- **Error Rate**: 20% (moderate error rate)
- **Use Case**: Error handling testing, alerting validation
- **Example**: `trazr-gen traces --enhanced --stress`

#### Chaos Mode (`--chaos`)
- **Error Rate**: 50% (high error rate)
- **Use Case**: Chaos testing, system resilience validation
- **Example**: `trazr-gen traces --enhanced --chaos`

#### Custom Mode (`--error-rate`)
- **Error Rate**: Configurable (0.0 to 1.0)
- **Use Case**: Specific testing scenarios
- **Example**: `trazr-gen traces --enhanced --error-rate 0.15`

### Error Control Summary

The enhanced summary shows the active error control mode:

```
=== Enhanced Trace Generation Summary ===
Traces generated: 1
Total spans: 5
Total logs: 11
Total events: 15
Total errors: 0
Logs by severity:
  INFO: 3
  DEBUG: 3
  WARN: 4
  ERROR: 1
Error control: Clean (no errors)
Using seed: 12345
Pattern: simple
Spans per trace: 5
==========================================
```

---

## Reproducible Generation with Seeds

TRAZR-GEN supports **reproducible generation** using configurable random seeds. This ensures consistent test data across runs:

### Seed Behavior

- **Fixed seed**: Use `--mock-seed <value>` for completely reproducible output
- **Timestamp-based**: Use `--mock-seed 0` or omit for dynamic generation
- **Seed tracking**: All telemetry includes `trazrgen.random.seed` in resource attributes

### Use Cases

- **Regression testing**: Ensure consistent test scenarios
- **Debugging**: Reproduce specific data patterns
- **Documentation**: Generate predictable examples
- **Performance testing**: Compare runs with identical data

### Example: Reproducible Generation

```sh
# Generate identical traces every time
trazr-gen traces --mock-seed 42 --enhanced --spans-per-trace 5

# Generate identical logs with mock data
trazr-gen logs --mock-seed 123 --logs 10 --mock-data

# Check resource attributes to see the seed used
trazr-gen traces --mock-seed 999 --enhanced --spans-per-trace 1
```

**Resource attributes will show:**
```
Resource Attributes:
  - service.name: trazr-gen
  - trazrgen.random.seed: 999
```

---

## Mock Data & Templates

TRAZR-GEN uses [gofakeit](https://github.com/brianvoe/gofakeit) for mock data. You can use any gofakeit template in attributes, headers, or log bodies.

**Example: Healthcare mock data from a config file:**
```yaml
otlp-attributes: 
  host.ip: '{{IPv4Address}}'
telemetry-attributes: 
  patient.name: '{{Name}}'
  patient.ssn: '{{SSN}}'
  patient.dob: '{{DateRange (ToDate "1924-01-01") (ToDate "2024-12-31")}}'
  encounter.procedure: "{{LoremIpsumSentence 10}}"
  encounter.type: '{{RandomString (SliceString "inpatient" "outpatient" "emergency")}}'
  credit.card.number: '{{CreditCard}}'

sensitive-data:
  - patient.name
  - patient.ssn
  - patient.dob
  - host.ip
  - credit.card.number
  - Body

logs:
  body: "{{ErrorDatabase}} - Patient Not Found: MRN{{Number 100000 999999}}"
```

**Sample output from a log collector:**
```log
Resource attributes:
-> host.ip: Str(129.108.24.106)
-> trazrgen.mock.data: Str(host.ip)
-> trazrgen.sensitive.data: Str(host.ip)
-> trazrgen.random.seed: Int(12345)
InstrumentationScope  
LogRecord #0
ObservedTimestamp: 1970-01-01 00:00:00 +0000 UTC
Timestamp: 2025-06-02 15:32:48.151832 +0000 UTC
SeverityText: Info
SeverityNumber: Info(9)
Body: Str(destination pointer is nil - Patient Not Found: MRN577540)
Attributes:
-> service.name: Str(trazr-gen)
-> credit.card.number: Str({UnionPay 341945079096768 02/30 916})
-> patient.name: Str(Elsie Barton)
-> patient.dob: Str(2010-04-17 06:22:29.470008037 +0000 UTC)
-> patient.ssn: Int(898828744)
-> encounter.procedure: Str(Voluptas recusandae dolores rerum nisi ducimus quasi qui ut accusamus.)
-> encounter.type: Str(inpatient)
-> trazrgen.sensitive.data: Str(patient.name, patient.ssn, patient.dob, credit.card.number, body)
-> trazrgen.mock.data: Str(credit.card.number, patient.name, patient.dob, patient.ssn, encounter.procedure, encounter.type, body)
```

---

## Examples

Generate 100 logs with random severity:
```sh
trazr-gen logs --logs 100 --severity-number "{{Number 1 24}}" --mock-data true
```

Generate metrics with custom attributes:
```sh
trazr-gen metrics --metrics 5 --otlp-attributes env=prod
```

Generate traces with a custom span duration:
```sh
trazr-gen traces --span-duration 500ms
```

**Output control examples:**
```sh
# Default behavior: detailed output + progress counter
trazr-gen logs --logs 5 --terminal-output

# Only progress counter (no detailed output)
trazr-gen logs --logs 5 --terminal-output --print-details=false

# Only detailed output (no progress counter)
trazr-gen logs --logs 5 --terminal-output --show-progress-counter=false

# Silent mode (no output at all)
trazr-gen logs --logs 5 --terminal-output --print-details=false --show-progress-counter=false

# Same unified flags work across all signals
trazr-gen metrics --metrics 10 --terminal-output --print-details=false
trazr-gen traces --traces 3 --terminal-output --print-details=false
```

**Enhanced trace examples:**
```sh
# Generate clean traces (no errors)
trazr-gen traces --enhanced --clean --spans-per-trace 10

# Generate stress traces for error handling testing
trazr-gen traces --enhanced --stress --pattern complex

# Generate chaos traces for chaos testing
trazr-gen traces --enhanced --chaos --spans-per-trace 15

# Generate complex traces with realistic patterns
trazr-gen traces --enhanced --spans-per-trace 15 --pattern complex --error-rate 0.1

# Generate reproducible traces for testing
trazr-gen traces --mock-seed 42 --enhanced --spans-per-trace 10 --pattern chain

# Generate traces with specific services and high concurrency
trazr-gen traces --enhanced --services "api-gateway,user-service,payment-service" --concurrency 5
```

---

## Documentation
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)

---

## Contributing
Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## License
TRAZR-GEN is licensed under the [Apache 2.0 License](LICENSE).

---

## Acknowledgments
- [OpenTelemetry Telemetrygen](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/telemetrygen)
- [gofakeit](https://github.com/brianvoe/gofakeit)

---

<p align="center">
  <sub>2025 Medoya LLC</sub>
</p>

---

New in this release: Graceful shutdown and bounded retry behavior

- Exporter creation now uses bounded exponential backoff:
  - Retries double from 500ms up to a maximum 10s delay
  - Total retry time is capped at 1 minute
  - On failure after 1 minute, the command exits with an error rather than retrying forever
- Shutdown sequencing improvements:
  - Traces: the tracer provider is shut down before the exporter to ensure spans are flushed
  - Logs and Metrics: a single exporter instance is shutdown once after work completes (no more per-worker shutdown calls)
- Practical effect: better UX when endpoints are unreachable and reduced risk of dropped spans on exit.

Environment variables (documentation only)

- TRAZR-GEN primarily supports configuration via CLI flags and YAML config files (see Configuration section above).
- Environment variable support will be expanded in a future update with a dedicated prefix and clearer key mapping. Until then, prefer CLI flags and YAML for reliable configuration.
- Example equivalents using flags today:
  - Log level: --log-level debug
  - OTLP endpoint: --otlp-endpoint localhost:4318
  - Use HTTP exporters: --otlp-http true
  - Resource attributes: --otlp-attributes service.name=trazr-gen,env=dev
