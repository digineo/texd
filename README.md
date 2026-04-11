# texd


[![Go Reference](https://pkg.go.dev/badge/github.com/digineo/texd.svg)](https://pkg.go.dev/github.com/digineo/texd)
[![Test, Lint, Release](https://github.com/digineo/texd/actions/workflows/test.yml/badge.svg)](https://github.com/digineo/texd/actions/workflows/test.yml)
[![Coverage](https://codecov.io/gh/digineo/texd/branch/master/graph/badge.svg)](https://codecov.io/gh/digineo/texd)
[![Go Report Card](https://goreportcard.com/badge/github.com/digineo/texd)](https://goreportcard.com/report/github.com/digineo/texd)
[![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/digineo/texd/master/LICENSE)


texd is a **Te**X **a**s (a) **S**ervice solution, designed for your internal document generation on your own servers.

## Features

- **Ad-hoc compilation** - Send `.tex` files via HTTP POST, get PDF back
- **Simple HTTP API** - No TeX distribution needed on client systems
- **Pluggable TeX distributions** - Use Docker containers for different TeX versions
- **Scalable architecture** - Deploy as local service, container, or CI pipeline
- **Reference store** - Cache commonly used assets to reduce bandwidth
- **Prometheus metrics** - Monitor document processing and queue status

## Quick Start

```console
# Local mode (requires local TeX installation)
$ texd

# Container mode (uses Docker images)
$ texd registry.gitlab.com/islandoftex/images/texlive:latest

# CI service mode (run texd in Docker)
$ docker run --rm -t -p localhost:2201:2201 ghcr.io/digineo/texd:latest
```

Then compile a document:

```console
$ curl -X POST \
    -F "input.tex=<input.tex" \
    -o "output.pdf" \
    "http://localhost:2201/render"
```

## Documentation

<!-- begin generated toc -->

- [Getting Started](./docs/getting-started.md) - Installation and operation modes
- **Configuration**
  - [CLI Options](./docs/cli-options.md) - Command-line options reference
- **API Reference**
  - [Render Endpoint](./docs/api-render.md) - Compile TeX documents to PDF
  - [Status Endpoint](./docs/api-status.md) - Server status and configuration
  - [Metrics](./docs/api-metrics.md) - Prometheus metrics
- **Features**
  - [Reference Store](./docs/reference-store.md) - Cache and reuse assets
  - [Web UI](./docs/web-ui.md) - Browser-based document compiler
- **More**
  - [History & Future](./docs/history.md) - Project background and roadmap
  - [Contributing](./docs/contributing.md) - How to contribute

<!-- end generated toc -->

## Contributing

Bug reports and feature requests are welcome at <https://github.com/digineo/texd/issues>.

Pull requests are welcome, even for minor fixes. For larger features, please open an issue first to discuss your proposal.

See [Contributing](./docs/contributing.md) for more details and related projects.

## License

MIT, © 2022, Dominik Menke, see file [LICENSE](./LICENSE)
