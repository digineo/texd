# texd

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
$ docker run --rm -t -p localhost:2201:2201 digineode/texd:latest
```

When using Gitlab CI, you can add this line to your `.gitlab-ci.yml`:

```yml
services:
  - name: digineode/texd:latest
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

## HTTP API

> TODO: formalize as OpenAPI spec?

### Render a document

To create a PDF document from an input `.tex` file, send a HTTP POST to the `/render` endpoint.
You may encode the payload as `multipart/form-data` or `application/x-www-form-encoded`, however
the latter is not recommended.

Assuming, you have a `input.tex` in the current directory, you can issue the following command
to send that file to your texd instance, and save the result in a file named `output.pdf`:

```console
$ curl -X POST \
    -F "input.tex=<input.tex" \
    -o "output.pdf" \
    "http://localhost:2201/render"
```

You can send multiple files (even in sub directories) as well:

```console
$ curl -X POST \
    -F "cv.tex=<cv.tex" \
    -F "chapters/introduction.tex=<chapters/introduction.tex" \
    -F "logo.pdf=<logo.pdf" \
    -o "vita.pdf" \
    "http://localhost:2201/render?input=cv.tex"
```

When sending multiple files, you should specify which one is the main input file (usually the one
containing `\documentclass`), using the `input=` query parameter. If you omit this parameter, texd
will try to guess the input file.

Please note that file names will be normalized, and files pointing outside the root directory
will be discarded entirely (i.e. `../input.tex` is NOT a valid file name). You can't do this:

```console
$ curl -X POST \
    -F "../input.tex=<input.tex" \
    -o "output.pdf" \
    "http://localhost:2201/render"
```

However, this is perfectly fine:

```console
$ curl -X POST \
    -F "input.tex=<../input.tex" \
    -o "output.pdf" \
    "http://localhost:2201/render"
```

<details><summary>Guessing the input file (click to show details)</summary>

- only filenames starting with alphanumeric character and ending in `.tex` are considered
  (`foo.tex`, `00-intro.tex` will be considered, but not `_appendix.tex`, `figure.png`)
- files in sub directories are ignored (e.g. `chapters/a.tex`)
- if only one file in the root directory remains, it is taken as main input
  - otherwise search for a file containing a line starting with:
    - either `%!texd` at the beginning of the file
    - or `\documentclass` somewhere in the first KiB
  - if no match, consider (in order):
    - `input.tex`
    - `main.tex`
    - `document.tex`

</details>

If no main input file can be determined, texd will abort with an error.

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
  in the texd command invocation, i.e. you can't select arbitrary images.

  If you provide an unknown image name, you will receive a 404 Not Found response. In *local* and
  *CI service* mode, this parameter only logged, but will otherwise be ignored.

- `errors=<detail level>` - tries to retrieve the compilation log, in case of compilation errors.
  Acceptable detail levels are:

  - *empty* (or `errors` completely absent), to return a JSON description (default)
  - `condensed`, to return only the TeX error message from the log file
  - `full`, to return the full log file as `text/plain` response

  The "condensed" form extracts only the lines from the error log which start with a `!`. Due to
  the way TeX works, these lines may not paint the full picture, as TeX's log lines generally don't
  exceed a certain line length, and wrapped lines won't get another `!` prefix.

  Note that this parameter changes the response content to a plain text file if you select `full`
  or `condensed`, and not a JSON response as in all other cases.

#### Successful response

If compilation succeeds, you'll receive a status 200 OK, with content type `application/pdf`, and
the PDF file as response body.

```http
HTTP/1.1 200 OK
Content-Type: application/pdf
Content-Length: 1234

%PDF/1.5...
```

#### Failure responses

If the request was accepted, but could not complete due to errors, you will by default receive a 422
Unprocessable Entity response with content type `application/json`, and an error description in
JSON format:

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/json
Content-Length: 154

{
  "error": "latexmk call failed with status 1",
  "category": "compilation",
  "output": "[truncated output log]"
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

- *reference* - texd could not find the provided reference store entries. The missing references
  are listed in the response; you need to repeat the request with those files included.

Additional fields, like `log` for compilation failures, might be present.

> Note: The JSON response is pretty-printed only for this README. Expect the actual response to
> be minified.

If you set `errors=full`, you may receive a plain text file with the compilation log:

<details><summary>Show response (click to open)</summary>

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: text/plain
Content-Length: 3156

This is XeTeX, Version 3.141592653-2.6-0.999993 (TeX Live 2021) (preloaded format=xelatex 2022.3.6)  12 MAR 2022 13:57
entering extended mode
 restricted \write18 enabled.
 %&-line parsing enabled.
... ommitting some lines ...
! LaTeX Error: File `missing.tex' not found.

Type X to quit or <RETURN> to proceed,
or enter new name. (Default extension: tex)

Enter file name:
! Emergency stop.
<read *>

l.3 \input{missing.tex}
                       ^^M
*** (cannot \read from terminal in nonstop modes)
```

</details>

For `errors=condensed`, you'll only receive the lines starting with `!` (with this prefix removed):

<details><summary>Show response (click to open)</summary>

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: text/plain
Content-Length: 59

LaTeX Error: File `missing.tex' not found.
Emergency stop.
```

</details>

### Status and Configuration

texd has a number of runtime configuration knobs and internal state variables, which may or may not
of interest for API consumers. To receive a current snapshot, query `/status`:

```console
$ curl -i http://localhost:2201/status
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Content-Length: 287

{
  "version":        "0.0.0",
  "mode":           "container",
  "images":         ["texlive/texlive:latest"],
  "timeout":        60,
  "engines":        ["xelatex","pdflatex","lualatex"],
  "default_engine": "xelatex",
  "queue": {
    "length":       0,
    "capacity":     16
  }
}
```

### Metrics

> TODO: not implemented yet

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

### Simple Web UI

You can try compiling TeX documents directly in your browser: Visit http://localhost:2201, and
you'll be greeted with a very basic, but functional UI.

Please note, that this UI is *not* built to work in every browser. It intentionally does not
use fancy build tools. It's just a simple HTML file, built by hand, using Bootstrap 5 for
aesthetics and Vue 3 for interaction. Both Bootstrap and Vue are bundled with texd, so you won't
need internet access for this to work.

If your browser does not support modern features like ES2022 proxies, `Object.entries`, `fetch`,
and `<object type="application/pdf" />` elements, you're out of luck. (Maybe upgrade your browser?)
Anyway, consider the UI only as demonstrator for the API.

## Reference store

texd has the ability to re-use previously sent material. This allows you to reduce the amount
of data you need to transmit with each render request. Following a back-on-the-envelope calculation:

- If you want to generate 1000 documents, each including a font with 400 kB in size, and a logo
  file with 100 kB in size, you will need to transmit 500 MB of the same two files in total.
- If you can re-use those two assets, you would only need to transmit them once, and use a reference
  hash for each subsequent request. The total then reduces 1×500 kB (complete assets for the first
  request) + 999×100 Byte (50 Byte per reference hash for subsequent requests) = 599.9 kB.

The feature in texd parlance is called "reference store", and you may think of it as a cache. It
saves files server-side (e.g. on disk) and retreives them on-demand, if you request such a file
reference.

A reference hash is simply the Base64-encoded SHA256 checksum of the file contents, prefixed with
"sha256:". (Canonically, we use the URL-safe alphabet without padding for the Base64 encoder, but
texd also accepts the standard alphabet, and padding characters are ignored in both cases.)

<details><summary>Python script to generate reference hashes (click to open details)</summary>

```python
#!/usr/bin/python3
"""
Save this file as texd-refhash.py, and call it like to:

    python3 texd-refhash.py FILE [FILE2 [...]]

This will print the texd file reference hash for FILE, FILE2, etc., each
on a separate line.
"""

from base64 import urlsafe_b64encode
from hashlib import sha256
from sys import argv

def refhash(filename: str):
  h = sha256()
  with open(filename, 'rb') as f:
    for block in iter(lambda: f.read(4096), b''):
        h.update(block)

  digest = urlsafe_b64encode(h.digest()).decode('ascii')
  return f'sha256:{digest}'

if __name__ == '__main__':
  for file in argv[1:]:
    print(refhash(file))
```

</details>

To *use* a file reference, you need to set a special content type in the request (calculated
using the script above), and include the reference hash instead of the file contents:

```console
$ python3 texd-refhash.py ../Hausschrift.otf | curl -X POST \
  -F "input.tex=<input.tex" \
  -F "Haussschrift.otf=@-;filename=Haussschrift.otf;type=application/x.tex;ref=use" \
  -o "output.pdf" \
  http://localhost:2201/render
```

For this to work, you need to starting the server with a valid `--reference-store` configuration.
Assuming `./refs` is an existing directory, you can issue the following command:

```console
$ texd --reference-store "dir://./refs"
```


## History

texd came to life because I've build dozens of Rails applications, which all needed to build PDF
documents in one form or another (from recipies, to invoices, order confirmations, reports and
technical documentation). Each server basically needed a local TeX installation (weighing in at
several 100 MB, up to several GB). Compiling many LaTeX documents also became a bottleneck for
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
bandwidth (assets like images are not easily transfer-compressible and their file size are orders of
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

This project is in its early stage.

Feel free to report bugs and feature request to <https://github.com/digineo/texd/issues>.

Pull requests are welcome, even minor ones for typo fixes. Before you start on a larger feature,
please create a proposal (in form of an issue) first.


## License

MIT, © 2022, Dominik Menke, see file [LICENSE](./LICENSE)
