# hawk.collector

![License](https://img.shields.io/badge/license-BSL--1.1-orange)
[![Go Report Card](https://goreportcard.com/badge/github.com/codex-team/hawk.collector)](https://goreportcard.com/report/github.com/codex-team/hawk.collector)

Powerful module that can handle most errors around the web.

# Build and run

Install RabbitMQ manually and build Hawk.collector

```bash
make build
./bin/hawk.collector
```

## Build for specific os

```bash
make build-linux
make build-windows
make build-darwin
```

or build all

```bash
make build-all
```

## Import RabbitMQ definitions

```bash
rabbitmqadmin import config/rabbit.definitions.json
```

## Run in Docker

```bash
make docker
```

# Data flows

## Request from errors catcher

The following structure represents data that go through the HTTP request (`POST` request to `'/'` with `Content-Type: application/json`)

| name         | type            | description                                         |
| ------------ | --------------- | --------------------------------------------------- |
| token        | string (base64) | JWT in base64 format                                |
| payload      | raw JSON        | Nested valid JSON                                   |
| catcherType  | string          | Type of the catcher (`errors/golang`, `errors/php`) |

JSON example
```
{
  "token": "...",
  "catcherType": "errors/golang",
  "payload": {
    "title": "Test exception",
    "timestamp": 1545203808
  }
}
```

## Request to upload sourcemap

The following structure represents data got through the HTTP request (`POST` request to `'/release'` with `Content-Type: multipart/form-data`)

### Form values
| name         | type                       | description                                         |
| ------------ | -------------------------- | --------------------------------------------------- |
| release      | string                     | Release name                                        |
| file         | multipart (optional)       | Content of the binary file                          |
| commits      | commitMessage (optional)   | Suspected commits                                   |

**commitMessage** has the following format:

| name         | type       | description                                         |
| ------------ | ---------- | --------------------------------------------------- |
| hash         | string     | Commit hash                                         |
| title        | string     | Commit description                                  |
| author       | string     | Commit author                                       |
| date         | string     | Commit date                                         |   

Authentication is made via `bearer` token.

cURL sending example
```
curl --request POST \
 -F 'release=1.0.1'\
 -F 'commits=[{"hash":"557940a440352d9d86ad5610f2e366aafb2729e4","title":"Add some stuff","author":"somebody@codex.so","date":"Wed May 6 13:37:00 2021 +0300"}]'\
 -F "repository=https://github.com/codex-team/hawk.api.nodejs"\
 -F file=@"main.min.js.map"\
 -H "Authorization: Bearer TOKEN" https://test.stage-k1.hawk.so/release
```

## Response message
HTTP response from the collector. It is provided as JSON with HTTP status code.

| name    | type   | description               |
| ------- | ------ | ------------------------- |
| code    | int    | internal error code       |
| error   | bool   | if the error was occurred |
| message | string | result details            |

For now there are two possible HTTP status codes: `200 (OK)` and `400 (Bad request)`.

Examples

```
{"error": true, "message": "Token is empty", "code": 200}
```

```
{"error": true, "message": "Invalid JSON format", "code": 400}
```

No body will be returned for the valid response (`200`).

## Websocket transport

Errors can be sent via websockets (for example with the help of [wscat](https://github.com/websockets/wscat) util).
```
~# wscat -c wss://test.stage-k1.hawk.so/ws                                                                                                                                                                                                              ✔  11720  20:53:23
connected (press CTRL+C to quit)
> f
< {"code":400,"error":true,"message":"Invalid JSON format"}
> {"token": "...","catcherType": "errors/golang","payload": {"title": "Test exception","timestamp": 1545203808}}
< {"code":200,"error":false,"message":"OK"}
```

# Message broker

For now we support RabbitMQ as a general AMQP broker.
We declare a durable **exchange** with `errors` name.
The valid payload JSON from `Request` structure goes directly to the exchange with the route specified by `catcherType` value.

# Environment variables

Basic configuration is taken from `.env` file.

| variable    | example value   | description               |
| ------- | ------ | ------------------------- |
| BROKER_URL   | amqp://guest:guest@localhost:5672/   | Connection URI to RabbitMQ  |
| EXCHANGE | errors | Basic exchange for errors             |
| RELEASE_EXCHANGE | release | Basic exchange for releases            |
| RETRY_NUMBER | 10 | Try to establish connection with broker for N times            |
| RETRY_INTERVAL | 4 | Wait N seconds before retry to establish connection with broker            |
| JWT_SECRET | qwerty | JWT token secret key            |
| MAX_REQUEST_BODY_SIZE | 20000000 | Maximum available HTTP body size for any request (in bytes)            |
| MAX_ERROR_CATCHER_MESSAGE_SIZE | 25000 | Maximum available HTTP body size for error request (in bytes)            |
| MAX_SOURCEMAP_CATCHER_MESSAGE_SIZE | 250000 | Maximum available HTTP body size for sourcemap request (in bytes)            |
| LISTEN | localhost:3000 | Listen host and port            |
| REDIS_URL | localhost:6379 | Redis address |
| REDIS_PASSWORD | password | Redis password |
| REDIS_DISABLED_PROJECT_SET | DisabledProjectsSet | Name of set that contains disabled projects IDs |
| REDIS_BLACKLIST_IP_SET | BlacklistIPsSet | Name of set that contains IPs blacklist |
| REDIS_ALL_IPS_MAP | AllIPsMap | Name of map with all IPs and their request counters |
| REDIS_CURRENT_PERIOD_MAP | CurrentPeriodMap | Name of map that contains IPs and their request counters for current period |
| BLOCKED_PROJECTS_UPDATE_PERIOD | 5s | Time interval to update blocked projects list |
| BLACKLIST_UPDATE_PERIOD | 15s | Time interval to update blacklist |
| BLACKLIST_THRESHOLD | 10000 | Amount of requests, which, when achieved, forces IP to get blocked |
| NOTIFY_URL | https://notify.bot.ifmo.su/u/ABCD1234 | Address to send alerts in case of too many requests |
| TOKEN_UPDATE_PERIOD | 10s | Time interval to update token cache |
| PROJECTS_LIMITS_UPDATE_PERIOD | 3600 | Time interval to update projects limits cache (in seconds) |
# Rate Limiting

Rate limiting is implemented using Redis to track and enforce request limits per project. The system supports configurable limits at the project, workspace and plan level.

## Configuration

Rate limits can be configured at multiple levels and applied in the following order (highest to lowest):

1. Project level - Individual project-specific limits
2. Workspace level - Limits that apply to all projects in a workspace
3. Plan level - Default limits from the workspace's tariff plan

## Implementation

Rate limits are tracked in `rate_limit` Redis set with the following pattern:

```go
// Key: "project_id" -> value: "timestamp:count"
// example: "6762b5db032b200023854b2c" -> "1737483572:5"
```

Each project's rate limit data contains:
- Timestamp of the current window
- Request count in the current window

### Rate Limit Parameters

Two main parameters control the rate limiting:

- `EventsLimit` - Maximum number of events allowed in the period
- `EventsPeriod` - Time window in seconds for the limit (in seconds)

## Configuration

Rate limits are fetched from MongoDB. You can find them in the `rateLimitSettings` field of the `plans,workspaces,projects` collections.

`rateLimitSettings` is object with two fields:
- `N` - Maximum number of events allowed in the period (`int64`)
- `T` - Time window in seconds for the limit (in seconds) (`int64`)

```json
{
  "rateLimitSettings": {
    "N": {
      "$numberLong": "15"
    },
    "T": {
      "$numberLong": "100"
    }
  }
}
```

Rate limits are automatically enforced for all incoming error and release events. No additional configuration is needed at the client level.

When a rate limit is exceeded, clients will receive a response like:

```json
{
  "code": 402,
  "error": true,
  "message": "Rate limit exceeded"
}
```

# License

Source code is available under **Business Source License 1.1 (BSL 1.1)**.
See [`LICENSE`](./LICENSE) for terms, including: