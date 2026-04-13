---
title: Status Endpoint
navTitle: Status Endpoint
section: API Reference
order: 2
description: Server status and configuration
---

# API Reference: Status and Configuration

texd has a number of runtime configuration knobs and internal state variables, which may or may not
be of interest for API consumers. To receive a current snapshot, query `/status`:

```console
$ curl -i http://localhost:2201/status
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Content-Length: 287

{
  "version":        "0.0.0",
  "mode":           "container",
  "images":         ["registry.gitlab.com/islandoftex/images/texlive:latest"],
  "timeout":        60,
  "engines":        ["xelatex","pdflatex","lualatex"],
  "default_engine": "xelatex",
  "queue": {
    "length":       0,
    "capacity":     16
  }
}
```
