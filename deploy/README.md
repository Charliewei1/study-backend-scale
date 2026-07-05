# Day 5: Docker container

This directory contains the Docker assets for the `shortlink` Go service.
The build context is `app/`, so run the commands from the repository root.

## Build

```sh
docker build -f deploy/Dockerfile -t shortlink:dev app
```

The Dockerfile is multi-stage:

1. `golang:1.24` downloads modules and builds a static Linux binary.
2. `gcr.io/distroless/static-debian12:nonroot` runs only that binary as a non-root user.

## Run

```sh
docker run --rm -p 8080:8080 shortlink:dev
```

Then check the health endpoint:

```sh
curl http://localhost:8080/healthz
```

The application listens on `PORT`, which defaults to `8080`. Override it only
when you also publish the matching container port:

```sh
docker run --rm -e PORT=9090 -p 9090:9090 shortlink:dev
```

## Appendix: single-stage comparison

`Dockerfile.single` is intentionally kept as a comparison target for the
lesson. It builds and runs in `golang:1.24`, so the final image includes the Go
toolchain, module cache, shell, and OS packages that are not needed at runtime.

Build it with:

```sh
docker build -f deploy/Dockerfile.single -t shortlink:single app
```

Compare image sizes:

```sh
docker images shortlink
```

The multi-stage image should be much smaller because only the compiled
`/shortlink` binary is copied into the distroless runtime image.
