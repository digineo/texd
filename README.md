# texd

texd is a TeXaS (TeX as (a) service) solution, designed for your internal document generation (i.e.,
on your own servers).

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
  distributions simulatenously)
- using HTTP enables redundancy and/or load balancing without much effort

## Early Development Note

The code in this repo is in a **VERY** early state. Most of it was created on a few rainy evenings.
Feel free to explore the code, but don't expect anything to work yet :-)

This Readme reflects how the end-goal *should* look like, based on my personal requirements. It is
in no way final or complete. Some described features might be removed, or extended upon.


## Operation Modes

texd is designed to be run/deployed in 2½ different ways:

### Local Mode

This is primarily for (local) testing and development. You download and start texd locally, provide
a TeX distribution, and texd will compile documents on your host machine.

To start texd in this mode, execute:

```console
$ texd
```

### Ephemeral Containers

Here, you still download and run texd locally, but document rendering will happen in an short-lived
Docker container, using a specific Docker image (`texlive/texlive:latest` will do just fine, but you
could easily build a smaller one using e.g. a Debian base image).

To run in container mode, run:

```console
$ texd texlive/texlive:latest
```

This will pull the specified image, if it doesn't exist yet. Note that you need to give texd
access to `/var/run/docker.sock`, in order to allow it to pull the image and create containers.

You may provide multiple image names and switch on a per-request basis (see HTTP API below). In
this case, the first image is used as default image:

```console
$ texd \
    texlive/texlive:latest \
    registry.gitlab.com/islandoftex/images/texlive:TL2014-historic \
    ghcr.io/yourcompany/texlive-prod
```

### CI Service

This runs texd within a Docker container, and is primarily targeted for CI pipelines, but can be a
viable alternative to the local mode. In fact, this mode is functionally equivalent to the
*local mode*, with the one exception (texd being packaged and started in a container).

To run texd as Docker service, use this command:

```console
$ docker run --rm -t -p localhost:2201:2201 dmke/texd:latest
```

When using Gitlab CI, you can add this line to your `.gitlab-ci.yml`:

```yml
services:
  - name: dmke/texd:latest
    alias: texd

variables:
  # reconfigure test application to use this endpoint
  # (this is specific to your application!)
  TEXD_ENDPOINT: http://texd:2201/render
```

This image is based on `texlive/texlive:latest`.

## CLI Options

Calling texd with options works in any mode; these commands are equivalent:

```console
$ texd -h
$ texd texlive/texlive:latest -h
$ docker run --rm -t dmke/texd:latest -h
```

- `--help`, `-h`

  Prints a short option listing and exits.

- `--version`, `-v`

  Prints version information and exists.

- `--listen-address=ADDR`, `-b ADDR` (Default: `:2201`)

  Specifies host address (optional) and port number for the HTTP API to bind to. Valid values are,
  among others:

  - `:2201` (bind to all addresses on port 2201)
  - `localhost:2201` (bind only to localhost on port 2201)
  - `[fe80::dead:c0ff:fe42:beef%eth0]:2201` (bind to a link-local IPv6 address on a specific interface)

- `--tex-engine=ENGINE`, `-X ENGINE` (Default: `xelatex`)

  TeX engine used to compile documents. Can be overridden on a per-request basis (see HTTP API below).
  Supported engines are `xelatex`, `lualatex`, and `pdflatex`.

- `--processing-timeout=DURATION`, `-t DURATION` (Default: `1m`)

  Maximum duration for a document rendering process before it is killed by texd. The value must be
  acceptable by Go's `ParseDuruation` function.

- `--parallel-jobs=NUM`, `-P NUM` (Default: number of cores)

  Concurrency level. PDF rendering is inherently single threaded, so limiting the document processing
  to the number of cores is a good start.

- `--queue-length=NUM`, `-q NUM` (Default: 1000)

  Maximum number of jobs to be enqueued before the HTTP API returns with an error.

- `--job-directory=PATH`, `-D PATH` (Default: OS temp directory)

  Place to put job sub-directories in. The path must exist and it must be writable.

> Note: This option listing might be outdated. Run `texd --help` to get the up-to-date listing.

## HTTP API

> TODO: formalize as OpenAPI spec?

### Render a document

To create a PDF document from an input `.tex` file, send a HTTP POST to the `/render` endpoint:

```console
$ curl -X POST \
    -F "input.tex=<input.tex" \
    http://localhost:2201/render
```

Please note that filenames will be normalized, and files pointing outside the root directory
will be discarded entirely (i.e. `../input.tex` is NOT a valid file name).

You can send multiple files (even in subdirectories) as well:

```console
$ curl -X POST \
    -F "cv.tex=<cv.tex" \
    -F "chapters/introduction.tex=<chapters/introduction.tex" \
    -F "logo.pdf=<logo.pdf" \
    http://localhost:2201/render\?input=cv.tex
```

If sending multiple files, you should specify which one is the main input file (usually the one
containing `\documentclass`), using the `input=` query parameter. If you omit this parameter, texd
will try to guess the input file:

- only filenames starting with alphanumeric character and ending in `.tex` are considered
  (`foo.tex`, `00-intro.tex` will be considered, but not `_appendix.tex`, `figure.png`)
- files in subdirectories are ignored (e.g. `chapters/a.tex`)
- if only one file in the root directory remains, it is taken as main input
  - otherwise search for a file containing a line starting with:
    - either `%!texd` at the beginning of the file
    - or `\documentclass` somethere in the first KiB
  - if no match, consider (in order):
    - `input.tex`
    - `main.tex`
    - `document.tex`

If no main input file can be determined, texd will abort with an error.

> Note (implementation detail): I would have preferred to use the *first* `.tex` file to be the
> main input file, but Go's form data parsing will convert the request body in to a key/value map
> (which are unordered and have a randomized order when iterating over their entries).

#### Responses

If compilation succeedes, you'll receive a status 200 OK, with content type `application/pdf`, and
the PDF file as response body.

```http
HTTP/1.1 200 OK
Content-Type: application/pdf
Content-Length: 1234

%PDF/1.5...
```

If the request was accepted, but could not complete due to errors, you will receive a 422
Unprocessable Entity response with content type `application/json`, and an error description in
JSON format:

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/json
Content-Length: 151

{
  "error": "latexmk call failed with status 1",
  "category": "compilation",
  "log": "[truncated output log]"
}
```

The fields `error` and `category` represent a short error description and an error category,
respectively.

Possible, known error categories are currently:

- *input* - one or more files are invalid (e.g. file was discarded after path normalization),
  or the main input file could not be determined.

- *compilation* - `latexmk` exited with an error (likely due to invalid or missing input files).

- *queue* - texd won't accept new render jobs, if its internal queue is at capacity. In this case
  wait for a few moments to give texd a chance to catch up and then try again.

Additional fields, like `log` for compilation failures, might be present.

> Note: The JSON response is pretty-printed only for this README. Expect the actual response to
> be minified.

#### URL Parameters

- `input=<filename>` - instructs texd to skip guessing main input file and use the specified one.
  The filename must be present in the body.

- `engine=<value>` - specifies which TeX engine to run. Supported engines are:

  - `xelatex` (default)
  - `lualatex`
  - `pdflatex`

  Note that the default can be changed with a CLI option (e.g. `--tex-engine=lualatex`).

- `image=<imagename>` - selects Docker image for document processing.

  This is only available in *ephemeral container* mode. The image name must match the ones listed
  in the texd command invokation, i.e. you can't select arbitrary images.

  If you provide an unknown image name, you will receive a 404 Not Found response. In *local* and
  *CI service* mode, this parameter only logged, but will otherwise be ignored.

### Status and Configuration

texd has a number of runtime configuration knobs and internal state variables, which may or may not
of interest for API consumers. To receive a current snapshot, query `/status`:

```console
$ curl -i http://localhost:2201/status
Content-Type: application/json
Content-Length: 178

{
  "version": "0.0.0",
  "mode:     "local",
  "images":  [],
  "engines": ["xelatex", "lualatex", "pdflatex"],
  "queue": {
    "length":   0,
    "capacity": 100,
  },
}
```

### Metrics

For monitoring, texd provides a Prometheus endpoint at `/metrics`:

```console
$ curl -i http://localhost:2201/metrics
Content-Type: text/plain; version=0.0.4; charset=utf-8

...
```

The metrics include Go runtime information, as well as texd specific metrics:

| Metric name | Type | Description |
|:------------|:-----|:------------|
| `texd_processed_total{status="success"}` | counter | Number of documents processed. |
| `texd_processed_total{status="failure"}` | counter | Number of rendering errors, including timeouts. |
| `texd_processed_total{status="rejected"}` | counter | Number of rejected requests, due to full job queue. |
| `texd_processed_total{status="aborted"}` | counter | Number of aborted requests, usually due to timeouts. |
| `texd_processing_duration_seconds` | histogram | Overview of processing time per document. |
| `texd_input_file_size_bytes{type=?}` | histogram | Overview of input file sizes. Type is either "tex" (for .tex, .cls, .sty, and similar files), "asset" (for images), "data" (for CSV files), or "other" (for unknown files) |
| `texd_output_file_size_bytes` | histogram | Overview of output file sizes. |
| `texd_job_queue_length` | gauge | Length of rendering queue, i.e. how many documents are waiting for processing. |
| `texd_job_queue_usage_ratio` | gauge | Queue capacity indicator (0.0 = empty, 1.0 = full). |
| `texd_info{version="0.0.0", mode="local", ...}` | constant | Various version and configuration information. |


Metrics related to processing also have an `engine=?` label indicating the TeX engine ("xelatex",
"lualatex", or "pdflatex"), and an `image=?` label indicating the Docker image.


## History

texd came to life because I've build dozens of Rails applications, which all needed to build PDF
documents in one form or another (from recipies, to invoices, order confirmations, reports and
technical documentation). Each server basically needed a local TeX installation (weighing in at
several 100 MB, upto serveral GB). Compiling many LaTeX documents also became a bottleneck for
applications running on otherwise modest hardware (or cloud VMs), as this process is also
computationally expensive.

Over time I've considered using alternatives for PDF generation (Prawn, HexaPDF, gofpdf, SILE, iText
PDF, to name but a few), and found that the quality of the rendered PDF is far inferior to the ones
generated by LaTeX. Other times, the licensing costs are  astronomical, or the library doesn't
support some layouting feature, or the library in an early alpha stage or already abandoned...

I'll admit that writing TeX templates for commercial settings is a special kind of pain-inducing
form of art. But looking back at using LaTeX for now over a decade, I still feel it's worth it.

## Future

Currently, texd is state-less: You send some files, and receive a PDF. If you want to render a
similar document, you need to send the some of the same files (or large parts of other files) again
and again.

It would be nice if clients could manage "projects" or "templates", i.e. a directory with a preset
list of static assets (like images, fonts, or document classes), and send only the dynamic files for
compilation. While I don't have reliable numbers (yet), I suspect this could save a lot of
bandwidth (assets like images are not easily transfer-compressable and their file size are orders of
magnitude larger then plaintext `.tex` files).

This would however introduce a state and managing it will introduce quite a lot of complexity on
both the client side (must know how to query and update a project) and the server side (merging
a static directory with a dynamic list of files, without cross-query contention, deciding if and
when it is OK to delete old projects, etc.)

---

Another whishlist item is asynchronous rendering: Consider rendering monthly invoices on the first
of each month; depending on the amount of customers/contracts/invoice positions, this can easily
mean you need to render a few thousand PDF documents.

Usually, the PDF generation is not time critical, i.e. they should finish in a reasonable amount of
time (say, within the next 6h to ensure timely delivery to the customer via email). For this to
work, the client could provide a callback URL to which texd sends the PDF via HTTP POST when
the rendering is finished.

Of course, this will also increase complexity on both sides: The client must be network-reachable
itself, an keep track of rendering request in order to associate the PDF to the correct invoice;
texd on the other hand would need a priority queue (processing async documents only if no sync
documents are enqueued), and it would need to store the callback URL somewhere.

---

In local mode (maybe even in container mode), it would be nifty to have a web UI to test the
document rendering.


## Contributing

This project is in its early stage.

Feel free to report bugs and feature request to <https://github.com/dmke/texd/issues>.

Pull requests are welcome, even minor ones for typo fixes. Before you start on a larger feature,
please create a proposal (in form of an issue) first.


## License

MIT, © 2022, Dominik Menke, see file [LICENSE][]
