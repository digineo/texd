# Operation Modes

texd is designed to be run/deployed in 2½ different ways:

## Local Mode

This is primarily for (local) testing and development. You download and start texd locally, provide
a TeX distribution, and texd will compile documents on your host machine.

To start texd in this mode, execute:

```console
$ texd
```

## Ephemeral Containers

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

## CI Service

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
