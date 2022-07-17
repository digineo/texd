# Reference store

texd has the ability to re-use previously sent material. This allows you to reduce the amount
of data you need to transmit with each render request. Following a back-of-the-envelope calculation:

- If you want to generate 1000 documents, each including a font with 400 kB in size, and a logo
  file with 100 kB in size, you will need to transmit 500 MB of the same two files in total.
- If you can re-use those two assets, you would only need to transmit them once, and use a reference
  hash for each subsequent request. The total then reduces 1×500 kB (complete assets for the first
  request) + 999×100 Byte (50 Byte per reference hash for subsequent requests) = 599.9 kB.

The feature in texd parlance is called "reference store", and you may think of it as a cache. It
saves files server-side (e.g. on disk) and retrieves them on-demand, if you request such a file
reference.

A reference hash is simply the Base64-encoded SHA256 checksum of the file contents, prefixed with
"sha256:". (Canonically, we use the URL-safe alphabet without padding for the Base64 encoder, but
texd also accepts the standard alphabet, and padding characters are ignored in both cases.)

To *use* a file reference, you need to set a special content type in the request, and include the
reference hash instead of the file contents. The content type must be `application/x.texd; ref=use`.

The resulting HTTP request should then look something like this:

```http
POST /render HTTP/1.1
Content-Type: multipart/form-data; boundary=boundary

--boundary
Content-Disposition: form-data; name=input.tex; filename=input.tex
Content-Type: application/octet-stream

[content of input.tex omitted]
--boundary
Content-Disposition: form-data; name=logo.pdf; filename=logo.pdf
Content-Type: application/x.texd; ref=use

sha256:p5w-x0VQUh2kXyYbbv1ubkc-oZ0z7aZYNjSKVVzaZuo=
--boundary--
```

For unknown reference hashes, texd will respond with an error, and list all unknown references:

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/json

{
  "category": "reference",
  "error": "unknown file references",
  "reference": [
    "sha256:p5w-x0VQUh2kXyYbbv1ubkc-oZ0z7aZYNjSKVVzaZuo="
  ]
}
```

In such a case, you can repeat you HTTP request, and change the `ref=use` to `ref=store` for
matching documents:

```http
POST /render HTTP/1.1
Content-Type: multipart/form-data; boundary=boundary

--boundary
Content-Disposition: form-data; name=input.tex; filename=input.tex
Content-Type: application/octet-stream

[content of input.tex omitted]
--boundary
Content-Disposition: form-data; name=logo.pdf; filename=logo.pdf
Content-Type: application/x.texd; ref=store

[content of logo.pdf omitted]
--boundary--
```

## Server configuration

By default, the reference store is not enabled. You must enable it explicitly, by providing
a command line flag. Assuming you have a local directory `./refs`, you instruct texd to use
this directory for references:

```console
$ texd --reference-store=dir://./refs
```

The actual syntax is `--reference-store=DSN`, where storage adapters are identified through and
configured with a DSN (*data source name*, a URL). Currently there are only handful implementations:

1. The `dir://` adapter ([docs][docs-dir]), which stores reference files on disk in a specified
   directory. Coincidentally, this adapter also provides an in-memory adapter (`memory://`),
   courtesy of the [spf13/afero][afero] package.

2. The `memcached://` adapter ([docs][docs-memcached]), which stores, you may have guessed it,
   reference files in a [Memcached][memcached] instance or cluster.

3. The `nop://` adapter ([docs][docs-nop]), which―for the sake of completeness sake―implements a
   no-op store (i.e. attempts to store reference file into is, or load files from it fail silently).
   This adapter is used as fallback if you don't configure any other adapter.

[docs-dir]: https://pkg.go.dev/github.com/digineo/texd/refstore/dir
[afero]: https://github.com/spf13/afero
[docs-memcached]: https://pkg.go.dev/github.com/digineo/texd/refstore/memcached
[memcached]: https://memcached.org/
[docs-nop]: https://pkg.go.dev/github.com/digineo/texd/refstore/nop

It is not unfeasible to imagine further adapters being available in the future, such as additional
key/value stores (`redis://`), object storages (`s3://`, `minio://`), or even RDBMS (`postgresql://`,
`mariadb://`).

## Data retention

texd supports three different retention policies:

1. `keep` (or `none`) will keep all file references forever. This is the default setting.
2. `purge-on-start` (or just `purge`) will delete file references once on startup.
3. `access` will keep an access list with LRU semantics, and delete file references, either if
   a max. number of items is reached, or if the total size of items exceeds a threshold, or both.

To select a specific retention policy, use the `--retention-policy` CLI flag:

```console
$ texd --reference-store=dir://./refs --retention-policy=purge
```

To configure the access list (`--retention-policy=access`), you can adopt the quota to your needs:

```
$ texd --reference-store=dir://./refs \
    --retention-policy=access \
    --rp-access-items=1000 \
    --rp-access-size=100MB
```

Notes:

- The default quota for the max. number of items (`--rp-access-items`) is 1000.
- The default quota for the max. total file size (`--rp-access-size`) is 100MB.
- Total file size is measured in bytes, common suffixes (100KB, 2MiB, 1.3GB) work as expected.
- To disable either limit, set the value to 0 (e.g. `--rp-access-items=0`).
- It is an error to disable both limits (in this case just use `--retention-policy=keep`).
- Currently, only the `dir://` (and `memory://`) adapter support a retention policy; the
 `memcached://` adapter delegates this responsibility to the Memcached server.
