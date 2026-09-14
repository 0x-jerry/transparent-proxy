# transparent-proxy

A small Go server that fetches a URL for you and streams the response back, with CORS enabled.

## Usage

```
http://localhost:8080/?url=https://proxy.com/xxx
```

```sh
go run ./cmd/transparent-proxy
curl 'http://localhost:8080/?url=https://example.com'
```

Open <http://localhost:8080/> in a browser for an interactive guide.

Only `GET` and `HEAD` are forwarded (`OPTIONS` answers CORS preflight); other methods return `405`.

> The path form `http://localhost:8080/https://proxy.com/xxx` is intentionally **not** supported — the target always goes in the `url` query parameter.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PROXY_ADDR` | `127.0.0.1:8080` | Listen address. Bound to loopback by default because this relay can reach internal networks. |
| `PROXY_TIMEOUT` | `30s` | Total upstream request timeout; `0` disables it (needed for long downloads). |
| `PROXY_INSECURE_TLS` | `false` | Skip upstream TLS verification. Logs a warning when enabled. |
| `PROXY_CORS` | `true` | Send permissive CORS headers and answer preflight requests. |

## Behavior

- The target must be an absolute `http` or `https` URL with no credentials.
- Upstream status codes and redirects are passed through unchanged.
- Request and response bodies are streamed; hop-by-hop headers are stripped.
- Transport failures return `502`, timeouts return `504`.

## Docker

```sh
docker build -t transparent-proxy .
docker run --rm -p 8080:8080 transparent-proxy
```

The image binds `0.0.0.0:8080` and includes CA certificates; override any setting with `-e`, e.g. `-e PROXY_TIMEOUT=0`.

## Development

```sh
go build ./...
go vet ./...
go test ./... -race
```
