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

A modular Go CLI application for generating OpenTelemetry logs, metrics, and traces. Designed for maintainability, scalability, and robust handling of sensitive data—ideal for regulated environments (healthcare, finance, etc.).

---


## Features

- Generate OpenTelemetry **logs**, **metrics**, and **traces**
- Flexible CLI and YAML config file support
- Mock data generation with [gofakeit](https://github.com/brianvoe/gofakeit)
- Sensitive data flagging for attributes and headers
- Human-friendly terminal output and machine-readable JSON logs
- Config diff display: see only non-default config at startup
- Distributed tracing and metrics with OpenTelemetry


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
- `--otlp-endpoint`    OTLP exporter endpoint
- `--service`          Service name
- `--log-level`        Log level (debug, info, warn, error)
- `--terminal-output`  Enable/disable terminal output instead of json log

See `trazr-gen [command] --help` or [config.yaml](https://github.com/medxops/trazr-gen/blob/main/config.yaml) for all options.

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
- [OpenTelemetry Telemetrygen](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/telemetrygen)
- [gofakeit](https://github.com/brianvoe/gofakeit)

---

<p align="center">
  <sub>2025 Medoya LLC</sub>
</p>