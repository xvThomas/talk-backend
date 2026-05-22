# mcp-ign-nav

MCP server providing French geographic tools powered by the [IGN Géoplateforme](https://data.geopf.fr/geocodage/) API.

## Tools

### `reverse_geocode`

Convert WGS84 geographic coordinates (longitude, latitude) into French addresses using the IGN reverse geocoding API.

**Input:**

| Field         | Type           | Required | Description                                              |
|---------------|----------------|----------|----------------------------------------------------------|
| `coordinates` | `[]Coordinate` | Yes      | List of coordinate pairs to reverse geocode (max 10)     |
| `index`       | `string`       | No       | Index to search: `address`, `poi`, or `parcel` (default: `address`) |
| `limit`       | `int`          | No       | Max results per coordinate, 1–50 (default: 1)            |

Each `Coordinate` has:
- `lon` (float64): Longitude in WGS84
- `lat` (float64): Latitude in WGS84

**Output:**

Returns a list of results, one per input coordinate, each containing matching features with: label, city, postcode, street, housenumber, type, score, and distance.

### `geocode`

Search for a French address or place name and return matching locations with their coordinates.

**Input:**

| Field      | Type      | Required | Description                                                      |
|------------|-----------|----------|------------------------------------------------------------------|
| `q`        | `string`  | Yes      | Search text: address, place name, or location to geocode         |
| `index`    | `string`  | No       | Index to search: `address`, `poi`, or `parcel` (default: `address`) |
| `limit`    | `int`     | No       | Maximum number of results, 1–50 (default: 5)                     |
| `postcode` | `string`  | No       | Filter by postal code                                            |
| `citycode` | `string`  | No       | Filter by INSEE city code                                        |
| `type`     | `string`  | No       | Filter by type: `housenumber`, `street`, `locality`, or `municipality` |
| `lon`      | `float64` | No       | Longitude to favor nearby results                                |
| `lat`      | `float64` | No       | Latitude to favor nearby results                                 |

**Output:**

Returns a list of matching locations, each with: label, city, postcode, street, housenumber, type, score, lon, lat, and context.

### `route`

Calculate a route between two points in France using the IGN Navigation API. Returns distance, duration, turn-by-turn steps, and optionally GeoJSON geometry.

**Input:**

| Field           | Type       | Required | Description                                                              |
|-----------------|------------|----------|--------------------------------------------------------------------------|
| `start`         | `string`   | Yes      | Start point as `longitude,latitude` (WGS84)                             |
| `end`           | `string`   | Yes      | End point as `longitude,latitude` (WGS84)                               |
| `resource`      | `string`   | No       | Routing resource: `bdtopo-osrm` (default) or `bdtopo-pgr`               |
| `profile`       | `string`   | No       | Routing profile: `car` (default) or `pedestrian`                         |
| `optimization`  | `string`   | No       | Optimization: `fastest` (default) or `shortest`                          |
| `intermediates` | `[]string` | No       | Ordered intermediate waypoints as `longitude,latitude` strings           |
| `avoidHighways` | `string`   | No       | Set to `true` to avoid highways (autoroutes)                             |

**Output:**

Returns: start, end, profile, optimization, total distance (m), total duration (s), bounding box, route portions with turn-by-turn steps (instruction, modifier, road name, road number, distance, duration), and optionally GeoJSON LineString geometry (when `GET_GEOJSON_GEOMETRY=true`).

## Configuration

No API key is required — the IGN Géoplateforme geocoding API is freely accessible under the [Licence Ouverte 2.0](https://www.etalab.gouv.fr/licence-ouverte-open-licence).

Optional environment variables (for MCP authentication, not for the IGN API):

| Variable                     | Description                          |
|------------------------------|--------------------------------------|
| `MCP_API_KEY`                | Static API key for MCP auth          |
| `MCP_OAUTH_AUTHORIZATION_SERVER` | OAuth authorization server URL  |
| `MCP_OAUTH_AUDIENCE`         | Expected JWT audience                |
| `MCP_OAUTH_SCOPES`           | Comma-separated OAuth scopes         |
| `MCP_BASE_URL`               | Public base URL of this server       |

Optional environment variables (tool behavior):

| Variable                | Default | Description                                                        |
|-------------------------|---------|--------------------------------------------------------------------|
| `GET_GEOJSON_GEOMETRY`  | `false` | When `true`, the route tool returns the full GeoJSON LineString geometry |

## Build & Run

```bash
# Build
make build

# Run (HTTP transport)
make run

# Run (stdio transport)
./bin/mcp-ign-nav --transport stdio

# Docker
make dockerize
docker run -p 8080:8080 mcp-ign-nav
```

## Development

```bash
make test       # Run tests
make vet        # Run go vet
make cover      # Run tests with coverage
make dev        # Hot reload with air
```
## Authentication

This server supports **X-API-Key** and **OAuth 2.0** authentication.  
See [docs/mcp-server-authentication.md](../docs/mcp-server-authentication.md) for configuration details.

## Security

This server includes built-in HTTP security hardening (rate limiting, path filtering, security headers, timeouts).  
See [docs/mcp-server-secured.md](../docs/mcp-server-secured.md) for configuration details.
