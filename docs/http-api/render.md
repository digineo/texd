# Render a document

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

## URL Parameters

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

## Successful response

If compilation succeeds, you'll receive a status 200 OK, with content type `application/pdf`, and
the PDF file as response body.

```http
HTTP/1.1 200 OK
Content-Type: application/pdf
Content-Length: 1234

%PDF/1.5...
```

## Failure responses

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
