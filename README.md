# hawk.catcher

[![Go Report Card](https://goreportcard.com/badge/github.com/codex-team/hawk.catcher)](https://goreportcard.com/report/github.com/codex-team/hawk.catcher)

Powerful module that can handle most errors around the web.

# Build and run

Install RabbitMQ manually and build Hawk.catcher

Before build enter the working directory

```bash
cd ./catcher
```

```bash
make
./catcher/bin/hawk.catcher
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

## Request

The following structure represents data got through the HTTP request (`POST` request with `Content-Type: application/json`)

| name         | type            | description                                         |
| ------------ | --------------- | --------------------------------------------------- |
| token        | string (base64) | JWT in base64 format                                |
| payload      | raw JSON        | Nested valid JSON                                   |
| catcher_type | string          | Type of the catcher (`errors/golang`, `errors/php`) |
| sender       | Sender          | Information about sender                            |

## Sender

Information about sender

| name | type   | description       |
| ---- | ------ | ----------------- |
| ip   | string | Sender IP address |

## Response

HTTP response from the catcher. It is provided as JSON with HTTP status code.

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
The valid payload JSON from `Request` structure goes directly to the exchange with the route specified by `catcher_type` value.

# Test

Run Hawk.catcher as described in the previous section.
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
  "sender": {
    "ip": "127.0.0.1"
  },
  "catcher_type": "errors/golang",
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

- Run catcher
- Open `tools/jsbomb/index.html` in browser
- Watch logs
