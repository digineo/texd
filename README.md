# texd

[![Go Reference](https://pkg.go.dev/badge/github.com/digineo/texd.svg)](https://pkg.go.dev/github.com/digineo/texd)
[![Test, Lint, Release](https://github.com/digineo/texd/actions/workflows/test.yml/badge.svg)](https://github.com/digineo/texd/actions/workflows/test.yml)
[![Coverage](https://codecov.io/gh/digineo/texd/branch/master/graph/badge.svg)](https://codecov.io/gh/digineo/texd)
[![Go Report Card](https://goreportcard.com/badge/github.com/digineo/texd)](https://goreportcard.com/report/github.com/digineo/texd)
[![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/digineo/texd/master/LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/digineode/texd)](https://hub.docker.com/r/digineode/texd)

texd is a TeXaS (TeX as (a) service) solution, designed for your internal document generation, i.e.
on your own servers.

It features:

- ad-hoc compilation,
- a simple HTTP API access,
- pluggable TeX distributions,
- ...

The idea behind texd is to provide a single, network-reachable compiler; sending a `.tex` file via
HTTP POST should then simply generate a PDF document. You won't need a TeX distribution on the
client system, just an HTTP client.

Several technologies make scaling in any dimension relatively easy:

- the TeX distribution is provided through Docker containers (this also allows using multiple
  distributions simultaneously)
- using HTTP enables redundancy and/or load balancing without much effort

## Documentation

The [latest documentation](./docs/index.md) can be found on GitHub.

If you have a running texd instance, head to http://localhost:2201/docs to view the documentation
for your instance.

## Related work

Of course, this project was not created in a void, other solutions exist as well:

- **latexcgi**, MIT license, [GitHub project][latexmk-gh], [Website][latexmk-web]

  Project description:

  > The TeXLive.net server (formally known as (LaTeX CGI server) (currently running at texlive.net)
  > accepts LaTeX documents via an HTTP POST request and returns a PDF document or log file in the
  > case of error.
  >
  > It is written as a perl script accepting the post requests via cgi-bin access in an apache
  > HTTP server.

- **Overleaf**, AGPL-3.0 license, [GitHub project][overleaf-gh], [Website][overleaf-web]

  Project description:

  > Overleaf is an open-source online real-time collaborative LaTeX editor. We run a hosted version
  > at www.overleaf.com, but you can also run your own local version, and contribute to the
  > development of Overleaf.

- **overleaf/clsi**, AGPL-3.0 license, [GitHub project][clsi-gh]

  Project description:

  > A web api for compiling LaTeX documents in the cloud
  >
  > The Common LaTeX Service Interface (CLSI) provides a RESTful interface to traditional LaTeX
  > tools (or, more generally, any command line tool for composing marked-up documents into a
  > display format such as PDF or HTML).

[latexmk-gh]: https://github.com/davidcarlisle/latexcgi
[latexmk-web]: https://davidcarlisle.github.io/latexcgi/
[overleaf-gh]: https://github.com/overleaf/overleaf
[overleaf-web]: https://www.overleaf.com
[clsi-gh]: https://github.com/overleaf/overleaf/tree/main/services/clsi

## Contributing

Please report bugs and feature request to <https://github.com/digineo/texd/issues>.

Pull requests are welcome, even minor ones for typo fixes. Before you start on a larger feature,
please create a proposal (in form of an issue) first.


## License

MIT, © 2022, Dominik Menke, see file [LICENSE](./LICENSE)
