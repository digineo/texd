---
title: Contributing
section: More
order: 2
description: How to contribute
---

# Contributing

Please report bugs and feature request to <https://github.com/digineo/texd/issues>.

Pull requests are welcome, even minor ones for typo fixes. Before you start on a larger feature,
please create a proposal (in form of an issue) first.

## Development Environment

To get started on the code base, you'll need the following:

- The [Go SDK](https://go.dev/dl). Using the most recent version is a good choice.
- Git (any recent version should do).
- GNU Make (optional, but highly recommended).
- Some form of TeX distribution, if you want to try to work on the compiler integration parts.

  This can either be a locally installed [TeXLive distribution](https://tug.org/texlive/),
  or a locally available TeX Docker image (for example [texlive/texlive:latest](https://hub.docker.com/r/texlive/texlive);
  obviously you also need [Docker](https://docker.com/))

To acquire a copy of the source code, run:

```console
$ git clone https://github.com/digineo/texd
```

Then `cd` into the `texd` directory. From there, you can use `make` to build, lint, and test
the source code:

```console
$ make build      # creates a local texd binary
$ make test       # runs test suite
$ make lint       # lints the Go sources
$ make run-local  # builds and starts a local version
```

Run `make help` (or just `make`) to get a list of targets.
