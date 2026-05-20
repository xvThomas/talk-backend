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
