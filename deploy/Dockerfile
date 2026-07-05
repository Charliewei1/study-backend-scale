# Build from the repository root with:
# docker build -f deploy/Dockerfile -t shortlink:dev app

FROM golang:1.24 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/shortlink \
    ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

ENV PORT=8080
EXPOSE 8080

COPY --from=builder /out/shortlink /shortlink

USER nonroot:nonroot
ENTRYPOINT ["/shortlink"]
