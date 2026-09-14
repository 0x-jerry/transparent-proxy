# transparent-proxy

A small Go server that fetches a URL for you and streams the response back, with CORS enabled.

## Usage

```
http://localhost:8080/https://proxy.com/xxx
```

```sh
go run ./cmd/transparent-proxy
curl 'http://localhost:8080/https://example.com'
```

Open <http://localhost:8080/> in a browser for an interactive guide.

Only `GET` and `HEAD` are forwarded (`OPTIONS` answers CORS preflight); other methods return `405`.

The target is the request path, so its own query string is preserved: `/https://example.com/search?q=go`.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PROXY_ADDR` | `127.0.0.1:8080` | Listen address. Bound to loopback by default because this relay can reach internal networks. |
| `PROXY_TIMEOUT` | `30s` | Total upstream request timeout; `0` disables it (needed for long downloads). |
| `PROXY_INSECURE_TLS` | `false` | Skip upstream TLS verification. Logs a warning when enabled. |
| `PROXY_CORS` | `true` | Send permissive CORS headers and answer preflight requests. |

## Behavior

- The target is the request path (leading `/` stripped) and must be an absolute `http` or `https` URL with no credentials.
- Redirects are followed and the final upstream response is returned.
- Request and response bodies are streamed; hop-by-hop headers are stripped.
- Remote identity headers (`Forwarded`, all `X-Forwarded-*`, `X-Real-IP`, `X-Client-IP`, `X-Originating-IP`, `X-Remote-IP`, `X-Remote-Addr`, `Client-IP`, `True-Client-IP`, `CF-Connecting-IP`, `Fastly-Client-IP`, `X-Cluster-Client-IP`) are stripped before the request is sent upstream.
- Upstream requests are sent with [`github.com/enetx/surf`](https://github.com/enetx/surf).
- Transport failures return `502`, timeouts return `504`.

## Docker

```sh
docker build -t transparent-proxy .
docker run --rm -p 8080:8080 transparent-proxy
```

The image binds `0.0.0.0:8080` and includes CA certificates; override any setting with `-e`, e.g. `-e PROXY_TIMEOUT=0`.

Or use the bundled `docker-compose.yml`, which pulls the published image from GHCR:

```sh
docker compose up
docker compose down
```

## Development

Requires Go 1.27 or newer.

```sh
go build ./...
go vet ./...
go test ./... -race
```
