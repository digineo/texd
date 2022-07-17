# CLI Options

Calling texd with options works in any [operation mode](./operation-modes.md);
these commands are equivalent:

```console
$ texd -h
$ texd texlive/texlive:latest -h
$ docker run --rm -t digineode/texd:latest -h
```

- `--help`, `-h`

  Prints a short option listing and exits.

- `--version`, `-v`

  Prints version information and exits.

- `--listen-address=ADDR`, `-b ADDR` (Default: `:2201`)

  Specifies host address (optional) and port number for the HTTP API to bind to. Valid values are,
  among others:

  - `:2201` (bind to all addresses on port 2201)
  - `localhost:2201` (bind only to localhost on port 2201)
  - `[fe80::dead:c0ff:fe42:beef%eth0]:2201` (bind to a link-local IPv6 address on a specific
    interface)

- `--tex-engine=ENGINE`, `-X ENGINE` (Default: `xelatex`)

  TeX engine used to compile documents. Can be overridden on a per-request basis (see HTTP API
  below). Supported engines are `xelatex`, `lualatex`, and `pdflatex`.

- `--compile-timeout=DURATION`, `-t DURATION` (Default: `1m`)

  Maximum duration for a document rendering process before it is killed by texd. The value must be
  acceptable by Go's `ParseDuruation` function.

- `--parallel-jobs=NUM`, `-P NUM` (Default: number of cores)

  Concurrency level. PDF rendering is inherently single threaded, so limiting the document
  processing to the number of cores is a good start.

- `--queue-wait=DURATION`, `-w DURATION` (Default: `10s`)

  Time to wait in queue before aborting. When <= 0, clients will immediately receive a "full queue"
  response.

- `--job-directory=PATH`, `-D PATH` (Default: OS temp directory)

  Place to put job sub directories in. The path must exist and it must be writable.

- `--pull` (Default: omitted)

  Always pulls Docker images. By default, images are only pulled when they don't exist locally.

  This has no effect when no image tags are given to the command line.

> Note: This option listing might be outdated. Run `texd --help` to get the up-to-date listing.
