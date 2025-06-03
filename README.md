<!-- Project Logo -->
<p align="center">
  <img src="docs/logo.png" alt="trazr-gen logo" width="1200"/>
</p>

[![Build Status](https://github.com/medxops/trazr-gen/actions/workflows/test.yaml/badge.svg)](https://github.com/medxops/trazr-gen/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/medxops/trazr-gen.svg)](https://pkg.go.dev/github.com/medxops/trazr-gen)
[![Go Report Card](https://goreportcard.com/badge/github.com/medxops/trazr-gen)](https://goreportcard.com/report/github.com/medxops/trazr-gen)
[![codecov](https://codecov.io/gh/medxops/trazr-gen/branch/main/graph/badge.svg)](https://codecov.io/gh/medxops/trazr-gen)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

---
`TRAZR-GEN` is a modular Go CLI application for generating observability, metrics, and trace data. Written in Go, it's designed for maintainability and scalability. A key feature is its robust handling of sensitive data for regulated environments. This includes generating realistic mock data and enabling the testing of infrastructure for redaction and filtering of sensitive information from live signals. This ultimately allows for safe and compliant data generation crucial for testing and analysis in industries like healthcare and finance.

---

## Features

- Generate OpenTelemetry **logs**, **metrics**, and **traces**
- Flexible CLI and YAML config file support
- Mock data generation with [gofakeit](https://github.com/brianvoe/gofakeit)
- Sensitive data flagging: Mark attributes and headers as sensitive to help troubleshoot, alert, and monitor for exposure or misuse in observability pipelines
- Human-friendly terminal output and machine-readable JSON logs
- Config diff display: see only non-default config at startup
- Distributed tracing and metrics with OpenTelemetry

---

## Installation

Trazr-Gen can be installed and run using various methods, depending on your operating system and preference.


### Homebrew (macOS & Linux)

If you're on macOS or Linux and have Homebrew installed, you can easily install Trazr-Gen.

1.  **Tap our Homebrew repository:**
    ```bash
    brew tap medxops/tap
    ```
2.  **Install Trazr-Gen:**
    ```bash
    brew install trazi-gen
    ```
    After installation, you can run `trazr-gen` commands directly in your terminal.

### Windows

For Windows users, you can install Trazr-Gen using Go, or build it from source.

#### Install via Go (Requires Go 1.20+ installed)

If you have Go installed on your Windows system, you can fetch and install `trazr-gen` directly:

```bash
go install [github.com/medxops/trazr-gen@latest](https://github.com/medxops/trazr-gen@latest)
```
### Docker

The quickest way to get started and run Trazr-Gen without installing Go locally is by using the official Docker image from GitHub Container Registry.

1.  **Pull the Docker Image:**
    ```bash
    docker pull ghcr.io/medxops/trazr-gen:latest
    ```
2.  **Run Trazr-Gen via Docker:**
    To generate data that sends to your OTLP collector (e.g., at `http://localhost:4318`), you can run:
    ```bash
    docker run --rm ghcr.io/medxops/trazr-gen:latest logs --logs 10 --otlp-endpoint [http://host.docker.internal:4318](http://host.docker.internal:4318)
    ```

### Run

Generate 10 logs with mock data with collector running on http://localhost:4318:

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

---

## Configuration

TRAZR-GEN supports configuration via CLI flags or a YAML config file. All options are shown here: [config.yaml](https://github.com/medxops/trazr-gen/blob/main/config.yaml)

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
- `--otlp-endpoint`    OTLP exporter endpoint
- `--service`          Service name
- `--log-level`        Log level (debug, info, warn, error)
- `--terminal-output`  Enable/disable terminal output

See `trazr-gen [command] --help` for all options.

---

## Mock Data & Templates

TRAZR-GEN uses [gofakeit](https://github.com/brianvoe/gofakeit) for mock data. You can use any gofakeit template in attributes, headers, or log bodies.

Example: Healthcare mock data from a config file:

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

sensitive-data: [patient.name, patient.ssn, patient.dob, host.ip, credit.card.number, Body]

logs:
  body: '{{ErrorDatabase}} - Patient Not Found: MRN{{Number 100000 999999}}'

```
Output from a log collector:

```log
Resource attributes:
-> host.ip: Str(129.108.24.106)
-> trazr.mock.data: Str(host.ip)
-> trazr.sensitive.data: Str(host.ip)
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
-> trazr.sensitive.data: Str(patient.name, patient.ssn, patient.dob, credit.card.number, body)
-> trazr.mock.data: Str(credit.card.number, patient.name, patient.dob, patient.ssn, encounter.procedure, encounter.type, body)
```

## Mock & Sensitive Data Marking

All attributes—including log body text—that use mock data will be listed under the key `trazr.mock.data`.

Additionally, all keys marked as sensitive data will be listed under the key `trazr.sensitive.data`. This is very useful for testing and validating backend signaling systems. See the example in the output above.

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

- [OpenTelemetry Telemetrygen](https://github.com/opentelemetry-collector-contrib/tree/main/cmd/telemetrygen)
- [gofakeit](https://github.com/brianvoe/gofakeit)

---

2025 Medoya LLC