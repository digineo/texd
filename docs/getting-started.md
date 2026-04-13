---
title: Getting Started
section: ""
order: 1
description: Installation and operation modes
---

# Getting Started

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
Docker container, using a specific Docker image (`registry.gitlab.com/islandoftex/images/texlive:latest`
will do just fine, but you could easily build a smaller one using e.g. a Debian base image).

To run in container mode, run:

```console
$ texd registry.gitlab.com/islandoftex/images/texlive:latest
```

This will pull the specified image, if it doesn't exist yet. Note that you need to give texd
access to `/var/run/docker.sock`, in order to allow it to pull the image and create containers.

You may provide multiple image names and switch on a per-request basis (see [HTTP API](api-render.md) below). In
this case, the first image is used as default image:

```console
$ texd \
    registry.gitlab.com/islandoftex/images/texlive:latest \
    registry.gitlab.com/islandoftex/images/texlive:TL2014-historic \
    ghcr.io/yourcompany/texlive-prod
```

### CI Service

This runs texd within a Docker container, and is primarily targeted for CI pipelines, but can be a
viable alternative to the local mode. In fact, this mode is functionally equivalent to the
*local mode*, with the one exception (texd being packaged and started in a container).

To run texd as Docker service, use this command:

```console
$ docker run --rm -t -p localhost:2201:2201 ghcr.io/digineo/texd:latest
```

The image `ghcr.io/digineo/texd:latest` is based on Debian Trixie with
some texlive packages installed from the Debian repositories (see this
[`Dockerfile`](https://github.com/digineo/texd/blob/master/.github/Dockerfile.base) for the current list). This also
means that the contained TeX distribution is [TeXlive 2024][].

> **Note:**
>
> If you want/need to run the container with non-root user ID (e.g. when
> started with sth. like `docker run --user=$(id -un):$(id -gn)`), make
> sure to also setup a `HOME` directory. Otherwise you'll likely encounter
> FontConfig caching errors.
>
> <details>
>   <summary>Example</summary>
>
>   ```console
>   $ mkdir -p texd/{home,jobs}
>   $ docker run --rm -t  \
>       -p localhost:2201:2201 \
>       --user $(id -un):$(id -gn) \
>       -e HOME=/texd/home \
>       -v $(pwd)/texd/home:/texd/home \
>       -v $(pwd)/texd/jobs:/texd/jobs \
>       ghcr.io/digineo/texd:latest \
>           --job-directory /texd/jobs
>   ```
> </details>

When using Gitlab CI, you can add this snippet to your `.gitlab-ci.yml`:

```yml
services:
  - name: ghcr.io/digineo/texd:latest
    alias: texd

variables:
  # reconfigure test application to use this endpoint
  # (this is specific to your application!)
  TEXD_ENDPOINT: http://texd:2201/render
```

[TeXlive 2024]: https://packages.debian.org/trixie/texlive
