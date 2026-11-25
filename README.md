
# IPQuery

A fast and efficient IP address query API built in Go. **IPQuery** provides geolocation, ISP, and risk assessment data for IP addresses with a clean API and containerized deployment options.

## Features

- **IP Geolocation**: Retrieve geographical information (country, city, coordinates, timezone) for any IP address
- **ISP Information**: Get autonomous system number (ASN), organization, and ISP details
- **Risk Assessment**: Identify VPNs, proxies, Tor exit nodes, datacenters, and calculate risk scores (via [AbuseIP**DB**](https://www.abuseipdb.com/))
- **REST API**: Simple HTTP/REST endpoints for IP queries
- **Docker Support**: Ready-to-deploy containerized application
- **GeoLite Integration**: Uses [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data/) free databases for accurate geolocation data
- **Terminal Output**: Aligned JSON with server-side ANSI colors for curl, HTTPie, Wget, and xh
- **Responsive Web UI**: IPv4/IPv6 search, JSON/table views, a location map, and built-in request examples

![Screenshot from 2026-01-07 12-06-39.png](assets/Screenshot%20from%202026-01-07%2012-06-39.png)

## Prerequisites

> [!IMPORTANT]
> Run locally with Go or Docker for development. For useful own-IP geolocation in
> production, the service must receive the public client IP. When using Caddy or
> another reverse proxy, configure `TRUSTED_PROXY_CIDRS` for the proxy network and
> forward the client address. Private IPs may have no GeoLite location data; the
> web UI displays an unavailable-location message instead of a map marker.

## Installation

### Binaries

Clone the repository:

```bash
git clone https://github.com/d0rsett/ipquery.git
cd ipquery
```

Build the project:

```bash
make build
```

Run the server:

```bash
./bin/ipquery
```

The API will be available at `http://localhost:8080`

### Docker

Build and run with Docker:

```bash
docker build -t ipquery:local .
docker run --rm --name ipquery-local -p 127.0.0.1:8080:8080 ipquery:local
```

> [!NOTE]
> That is a simple quick way to run the server, without the risk assessment enricher. For that you need to get 
> an API KEY from [AbuseIP**DB**](https://www.abuseipdb.com/) and set it as environment variable `ABUSEIPDB_API_KEY`.

Open [http://localhost:8080/](http://localhost:8080/) in a browser, or run
`curl http://localhost:8080/` for colored terminal output. The command above binds
only to localhost. For a server deployment, publish the port on the required
interface or expose the API through a reverse proxy.

### Docker Compose (with Caddy)

The deployment files below are inherited upstream examples and reference upstream
images. To run this fork's changes, first build this checkout and replace the API
service's image with `ipquery:local` (or your own image tag) before starting Compose.
Do the same for the Kong example. No upstream image includes local changes.

Use the included deployment files:

```bash
cd deploy/caddy
docker compose up -d
```

```yaml
version: "3.8"
name: ipquery

services:
  api:
    image: akyriako78/ipquery:0.1.17
    environment:
      LISTEN_ADDR: ":8080"
      TRUSTED_PROXY_CIDRS: "172.30.0.0/16"
      GEOLITE2_ASN: "/geolite/GeoLite2-ASN.mmdb"
      GEOLITE2_CITY: "/geolite/GeoLite2-City.mmdb"
      ABUSEIPDB_API_KEY: "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
    networks:
      ipqnet:
        ipv4_address: 172.30.0.10

  caddy:
    image: caddy:2-alpine
    depends_on:
      - api
    ports:
      - "8080:80"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
    networks:
      ipqnet:
        ipv4_address: 172.30.0.2

networks:
  ipqnet:
    driver: bridge
    ipam:
      config:
        - subnet: 172.30.0.0/16

```

> [!NOTE]
> Replace `ABUSEIPDB_API_KEY` with your very own key. **If none is provided, no risk assessment will be performed**.

### Docker Compose (with Kong)

Use the included deployment files:

```bash
cd deploy/kong
docker compose up -d
```

```yaml
version: "3.8"
name: ipquery_gw

services:
  api:
    image: akyriako78/ipquery:0.1.18
    environment:
      LISTEN_ADDR: ":8080"
      GEOLITE2_ASN: "/geolite/GeoLite2-ASN.mmdb"
      GEOLITE2_CITY: "/geolite/GeoLite2-City.mmdb"
      ABUSEIPDB_API_KEY: "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
    networks:
      - kong-net

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: kong
      POSTGRES_USER: kong
      POSTGRES_PASSWORD: kongpass
    volumes:
      - kong_pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U kong -d kong"]
      interval: 5s
      timeout: 5s
      retries: 30
    networks:
      - kong-net

  kong-migrations:
    image: kong:3.7
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      KONG_DATABASE: postgres
      KONG_PG_HOST: postgres
      KONG_PG_DATABASE: kong
      KONG_PG_USER: kong
      KONG_PG_PASSWORD: kongpass
    command: kong migrations bootstrap
    restart: "no"
    networks:
      - kong-net

  kong:
    image: kong:3.7
    depends_on:
      kong-migrations:
        condition: service_completed_successfully
    environment:
      KONG_DATABASE: postgres
      KONG_PG_HOST: postgres
      KONG_PG_DATABASE: kong
      KONG_PG_USER: kong
      KONG_PG_PASSWORD: kongpass

      # Proxy (traffic)
      KONG_PROXY_LISTEN: "0.0.0.0:8000, 0.0.0.0:8443 ssl"
      # Admin API (keep internal if you want, see notes below)
      KONG_ADMIN_LISTEN: "0.0.0.0:8001, 0.0.0.0:8444 ssl"
      # Kong Manager UI
      KONG_ADMIN_GUI_LISTEN: "0.0.0.0:8002"
      KONG_ADMIN_GUI_URL: "http://localhost:8002"

      # OPTIONAL: convenient in dev; restrict/disable in prod
      KONG_ADMIN_GUI_AUTH: "basic-auth"
      KONG_PASSWORD: "adminpass"

      # recommended logs to stdout for docker
      KONG_PROXY_ACCESS_LOG: /dev/stdout
      KONG_ADMIN_ACCESS_LOG: /dev/stdout
      KONG_PROXY_ERROR_LOG: /dev/stderr
      KONG_ADMIN_ERROR_LOG: /dev/stderr

    ports:
      - "8000:8000"   # proxy (http)
      - "8443:8443"   # proxy (https)
      - "8001:8001"   # admin api (http)
      - "8444:8444"   # admin api (https)
      - "8002:8002"   # kong manager (gui)
    networks:
      - kong-net

  kong-config-import:
    image: kong:3.7
    depends_on:
      kong:
        condition: service_started
    networks:
      - kong-net
    volumes:
      - ./kong.yml:/kong/kong.yml:ro
    environment:
      KONG_DATABASE: postgres
      KONG_PG_HOST: postgres
      KONG_PG_DATABASE: kong
      KONG_PG_USER: kong
      KONG_PG_PASSWORD: kongpass
    command: kong config db_import /kong/kong.yml
    restart: "no"

volumes:
  kong_pg_data:

networks:
  kong-net:
    driver: bridge
```

![Screenshot from 2026-01-07 12-10-05.png](assets/Screenshot%20from%202026-01-07%2012-10-05.png)

> [!IMPORTANT]
> Make sure you provide a strong passwords for `KONG_PG_PASSWORD`, `KONG_PASSWORD` and `POSTGRES_PASSWORD`.

> [!NOTE]
> This is a more elaborate setup, but give you the flexibility to take advantage of all the features of Kong Gateway 
> like securing your exposed API endpoints with API Keys and/or impose rate limiting. This docker compose is not
> production-ready, but nevertheless a good starting point to get you going.

## API Endpoints

The following table describes the default configuration (`IPQUERY_DEFAULT_FORMAT=auto`):

| Route | Browser | curl |
| --- | --- | --- |
| `/` | Web interface | Colored, aligned own-IP metadata |
| `/own` | Plain IP address | Plain IP address |
| `/own/all` | Readable JSON for your IP | Colored, aligned own-IP metadata |
| `/lookup/{ip}` | Readable JSON for the specified IPv4/IPv6 | Colored, aligned metadata |
| `/health` | `ok` | `ok` |

### Web interface

The web interface accepts IPv4 and IPv6 addresses and offers JSON and table views.
Expand **How to use the API** for English curl examples. Commands use the current
deployment URL, including a reverse proxy path prefix.

Below **1024 px**, results and the map stack in one column and the page scrolls.
At 1024 px and above, they appear side by side. On narrow phones, the search input
and button also stack; long JSON and command examples scroll inside their panels.
Short desktop windows scroll as needed, including when the API help is expanded.
Location data is approximate. IPs without GeoLite location data show a message
instead of a misleading marker at zero latitude and longitude. The map uses
MapLibre and OpenStreetMap tiles; external map resources require network access.

### `/`

Returns your own IP metadata to CLI and programmatic clients, using the same
schema as `/own/all`. Browser navigation continues to open the existing web UI.
Use `/?format=pretty` or `Accept: application/json` to request data in a browser.

### Response formatting

Formatting is available on `/`, `/own/all`, and `/lookup/{ip}`. IPv4 and IPv6
use the same response schema and formatting. `/own` remains a plain IP string
and `/health` remains `ok`; their query parameters do not change these contracts.

```bash
# Colorful, aligned metadata for your own IP (after deploying this version):
curl ip.y00u.ru

# The same behavior locally:
curl http://localhost:8080

# Explicit response modes:
curl 'http://localhost:8080/lookup/8.8.8.8?format=json'
curl 'http://localhost:8080/lookup/8.8.8.8?format=pretty'
curl 'http://localhost:8080/lookup/2001:4860:4860::8888?format=ansi'
curl 'http://localhost:8080/own/all?format=auto'

# Scripts: request valid JSON explicitly, including when piping or saving output:
curl -sS -H 'Accept: application/json' http://localhost:8080/own/all
curl -sS 'http://localhost:8080/?format=json' > ip.json

# Other supported CLI clients:
http http://localhost:8080/own/all
wget -qO- http://localhost:8080/own/all
```

| `format` | Output |
| --- | --- |
| `json` | Compact standard JSON, no ANSI |
| `pretty` | Readable JSON with aligned colons, no ANSI |
| `ansi` | Pretty JSON with server-generated terminal colors |
| `auto` | Choose using `User-Agent` and `Accept` |

An explicit `format` overrides the configured default and automatic detection.
An explicit `format=auto` runs detection even when the configured default is
another mode. Unknown, empty, or repeated format values return HTTP 400 on the
metadata endpoints. Existing plain-text error responses are preserved.

In auto mode, curl, HTTPie, GNU Wget (`Wget/`), and xh receive ANSI when
`Accept` permits `text/plain`. A positive explicit `Accept: application/json`
with a quality at least as high as the text alternatives requests standard JSON,
including for CLI clients. Media quality weights and specific exclusions such as
`text/plain;q=0` are respected when deciding whether to send ANSI. Browser API
requests receive pretty JSON unless JSON is explicitly preferred; other clients
receive standard JSON. The browser landing page remains available on `/`.

HTTP cannot tell whether curl is attached to a terminal, redirected to a file,
or piped into another program. Scripts should always set `format=json` or
`Accept: application/json`; a bare curl request intentionally defaults to colors.

Pretty output preserves struct field order and aligns keys separately within
each object. Arrays and nested objects are indented, empty containers stay
`{}` and `[]`, and every metadata response ends with a newline. Colors reset
after each key or scalar: cyan keys, green strings, magenta numbers, blue
booleans, and gray null; punctuation retains the terminal's default color.
Common combining and East Asian wide characters are accounted for in key
alignment; exact display width of emoji sequences can vary by terminal.

ANSI responses use `Content-Type: text/plain; charset=utf-8`. Standard and pretty
JSON use `application/json; charset=utf-8`. Responses vary by `User-Agent` and
`Accept` for proxy caches. Caddy and other reverse proxies should forward these
headers and preserve query strings. Existing `TRUSTED_PROXY_CIDRS` behavior for
resolving the client IP is unchanged; trust only your actual proxy networks.

### Formatting configuration

| Environment variable | Default | Meaning |
| --- | --- | --- |
| `IPQUERY_DEFAULT_FORMAT` | `auto` | `auto`, `json`, `pretty`, or `ansi` |
| `IPQUERY_COLOR_ENABLED` | `true` | Global color switch; `false` converts ANSI mode to pretty JSON, even when explicitly requested |
| `IPQUERY_ALIGN_KEYS` | `true` | Align colons within each object in pretty/ANSI modes |
| `IPQUERY_JSON_INDENT` | `2` | Spaces per nesting level, from 0 to 16 |

Invalid configuration fails at startup. These options do not change compact JSON
or the IP lookup/enrichment logic and require no additional dependencies.

```bash
docker build -t ipquery:local .
docker run --rm -p 8080:8080 \
  -e IPQUERY_DEFAULT_FORMAT=auto \
  -e IPQUERY_COLOR_ENABLED=true \
  -e IPQUERY_ALIGN_KEYS=true \
  -e IPQUERY_JSON_INDENT=2 \
  ipquery:local
```

Set the same variables under the API service's `environment` section in Docker
Compose. Existing prebuilt upstream image tags do not include these changes;
build this checkout and point your deployment at the resulting image.

### Development checks and architecture

```bash
go test ./...
go vet ./...
go build -o bin/ipquery ./cmd
docker build -t ipquery:local .
```

`api/lookup_client.go` resolves client IPs and the GeoLite/AbuseIPDB enrichers
populate `LookupResult`. The HTTP handlers serialize that unchanged result once
with `encoding/json`, then `response` handles ordered formatting and content
negotiation. `cmd` loads environment configuration and registers the existing
routes. Formatting tests cover JSON types, escaping, precision, Unicode, nesting,
colors, alignment, and negotiation. Route tests exercise the actual router and
bundled GeoLite databases, including IPv6 and trusted proxy requests; they do not
call AbuseIPDB or require an API key.

### `/own`

Resolves your own IP address, returns only the IP as `string`

### `/own/all`

Resolves your own IP address, returns all metadata found in MaxMind GeoLite2 databases as `JSON`, e.g.:

```json
{
  "ip": "141.98.XXX.XXX",
  "isp": {
    "asn": "AS39351",
    "org": "31173 Services AB",
    "isp": "31173 Services AB"
  },
  "location": {
    "country": "Denmark",
    "country_code": "DK",
    "city": "Copenhagen",
    "state": "Capital Region",
    "zipcode": "XXXX",
    "latitude": 55.6761,
    "longitude": 12.5683,
    "timezone": "Europe/Copenhagen",
    "localtime": "2026-01-07T12:06:30+01:00"
  },
  "risk": {
    "abuse_confidence_score": 0,
    "usage_type": "Fixed Line ISP",
    "is_tor": false,
    "total_reports": 1,
    "number_of_users_reported": 1,
    "last_reported_at": "2025-11-16T04:30:39Z"
  }
}
```

> [!NOTE]
> The MaxMind GeoLite2 databases are prebaked in the container image. If you need to provide your own load
> them in volumes and configure `GEOLITE2_ASN` and `GEOLITE2_CITY` environment variables accordingly.

### `/lookup/{ip}`

Resolves the requested IP address, returns all metadata found in MaxMind GeoLite2 databases as `JSON`.

> [!IMPORTANT] 
> Risk (stanza `risk`) is assessed only if a valid `ABUSEIPDB_API_KEY` is provided. 
> AbuseIP**DB** request limits depend on your subscription; check your account before enabling it for a public deployment.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues, questions, or suggestions, please open an issue on the GitHub repository.
