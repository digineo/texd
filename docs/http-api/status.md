# Status and Configuration

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
