# hawk.collector

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

## Request to upload sourcemap

The following structure represents data got through the HTTP request (`POST` request to `'/sourcemap'` with `Content-Type: multipart/form-data`)

```
curl -F release="1.0.1" -F file=@"sourcemap.js" -H "Authorization: Bearer Q29kZVggdGVhbQo=" http://localhost:3000/sourcemap
```

### Form values
| name         | type            | description                                         |
| ------------ | --------------- | --------------------------------------------------- |
| release      | string          | Release name                                        |

### Form files
Set of files with filenames encoded with [multipart/form-data](https://tools.ietf.org/html/rfc2388)

## Response message
HTTP response from the collector. It is provided as JSON with HTTP status code.

| name    | type   | description               |
| ------- | ------ | ------------------------- |
| error   | bool   | if the error was occurred |
| message | string | result details            |

For now there are two possible HTTP status codes: `200 (OK)` and `400 (Bad request)`.

Examples

```
{"error": true, "message": "Token is empty"}
```

```
{"error": true, "message": "Invalid JSON format"}
```

No body will be returned for the valid response (`200`).


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
| SOURCEMAP_EXCHANGE | release/javascript | Basic exchange for sourcemaps            |
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

# Test

Run Hawk.collector as described in the previous section.
Run load tests

```python
python3 -m venv venv
source venv/bin/activate
pip install -r tools/requirements.txt
python3 tools/bomber.py
```

You can also send requests manually via insomnia or cURL to the `http://localhost:3000/` with the following JSON body.

```json
{
  "token": "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9",
  "catcherType": "errors/golang",
  "payload": {
    "title": "Field `blocks` is missing",
    "timestamp": 1545203808,
    "severity": 16,
    "backtrace": [
      {
        "file": "/var/www/codex/vendor/codex-team/editor.js/EditorJS/EditorJS.php",
        "line": 77,
        "source code": [
          {
            "line number": 76,
            "content": "         if (!isset($data['blocks'])) {"
          },
          {
            "line number": 77,
            "content": "             throw new EditorJSException('Field `blocks` is missing');"
          },
          {
            "line number": 78,
            "content": "         }"
          }
        ]
      },
      {
        "file": "/var/www/codex/application/classes/Controller/Articles/Index.php",
        "called line": "191",
        "source code": [
          {
            "line number": "190",
            "content": "     {"
          },
          {
            "line number": "191",
            "content": "         $editor = new EditorJS($content, Model_Article::getEditorConfig());"
          },
          {
            "line number": "192",
            "content": "         $blocks = $editor->getBlocks();"
          }
        ]
      }
    ],
    "get": {},
    "post": {
      "text": "Hello, World!",
      "is_published": false
    },
    "headers": {},
    "source release": ""
  }
}
```

## WebSocket testing

- Run collector
- Open `tools/jsbomb/index.html` in browser
- Watch logs
