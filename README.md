# hawk.catcher
Powerful module that can handle most errors around the web.

# Build and run

Install rabbitMQ manually and build Hawk.catcher
```
make
./bin/hawk.catcher
```

# Test

Run Hawk.catcher as described in the previous section.
Run load tests
```python
python3 -m venv venv
source venv/bin/activate
pip install -r tools/requirements.txt
python3 tools/bomber.py
```

You can also send requests manually via insomnia or cURL to the `http://localhost:3000/catcher` with the following JSON body.

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
    "get": {
    },
    "post": {
      "text": "Hello, World!",
      "is_published": false
    },
    "headers": {
    },
    "source release": ""
	}
}
```

# Run in Docker

```
make docker-build
make docker-run
```