# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/transparent-proxy ./cmd/transparent-proxy

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY --from=build /out/transparent-proxy /usr/local/bin/transparent-proxy

# Must bind beyond loopback to be reachable through a published port.
ENV PROXY_ADDR=0.0.0.0:8080
EXPOSE 8080
USER nobody
ENTRYPOINT ["transparent-proxy"]
