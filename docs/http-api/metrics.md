# Metrics

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
| `texd_input_file_size_bytes{type=?}` | histogram | Overview of input file sizes. Type is either "tex" (for .tex, .cls, .sty, and similar files), "asset" (for images and fonts), "data" (for CSV files), or "other" (for unknown files) |
| `texd_output_file_size_bytes` | histogram | Overview of output file sizes. |
| `texd_job_queue_length` | gauge | Length of rendering queue, i.e. how many documents are waiting for processing. |
| `texd_job_queue_usage_ratio` | gauge | Queue capacity indicator (0.0 = empty, 1.0 = full). |
| `texd_info{version="0.0.0", mode="local", ...}` | constant | Various version and configuration information. |


Metrics related to processing also have an `engine=?` label indicating the TeX engine ("xelatex",
"lualatex", or "pdflatex"), and an `image=?` label indicating the Docker image.
