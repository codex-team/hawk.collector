package sentry

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestComposeTitle(t *testing.T) {
	t.Run("from exception type and value", func(t *testing.T) {
		event := gjson.Parse(`{"exception":{"values":[{"type":"Error","value":"Something went wrong"}]}}`)
		assert.Equal(t, "Error: Something went wrong", ComposeTitle(event))
	})

	t.Run("missing exception data", func(t *testing.T) {
		event := gjson.Parse(`{}`)
		assert.Equal(t, "Unknown: ", ComposeTitle(event))
	})

	t.Run("from message if exception missing", func(t *testing.T) {
		event := gjson.Parse(`{"message":"message"}`)
		assert.Equal(t, "message", ComposeTitle(event))
	})

	t.Run("exception preferred over message", func(t *testing.T) {
		event := gjson.Parse(`{"exception":{"values":[{"type":"Error","value":"Something went wrong"}]},"message":"message"}`)
		assert.Equal(t, "Error: Something went wrong", ComposeTitle(event))
	})
}

func TestComposeBacktrace(t *testing.T) {
	t.Run("complete frame data", func(t *testing.T) {
		event := gjson.Parse(`{
			"exception":{"values":[{"stacktrace":{"frames":[{
				"filename":"test.js","lineno":10,"colno":5,"function":"testFunction",
				"vars":{"param1":"value1"},
				"context_line":"const x = 1;",
				"pre_context":["// comment"],
				"post_context":["console.log(x);"]
			}]}}]}
		}`)
		bt := ComposeBacktrace(event)
		require.Len(t, bt, 1)
		assert.Equal(t, "test.js", bt[0].File)
		assert.Equal(t, 10, bt[0].Line)
		require.NotNil(t, bt[0].Column)
		assert.Equal(t, 5, *bt[0].Column)
		assert.Equal(t, "testFunction", bt[0].Function)
		assert.Equal(t, []string{"param1=value1"}, bt[0].Arguments)
		assert.Equal(t, []SourceCodeLine{
			{Line: 9, Content: "// comment"},
			{Line: 10, Content: "const x = 1;"},
			{Line: 11, Content: "console.log(x);"},
		}, bt[0].SourceCode)
	})

	t.Run("missing frame data uses instruction_addr", func(t *testing.T) {
		event := gjson.Parse(`{"exception":{"values":[{"stacktrace":{"frames":[{"instruction_addr":"0x123"}]}}]}}`)
		bt := ComposeBacktrace(event)
		require.Len(t, bt, 1)
		assert.Equal(t, BacktraceFrame{File: "0x123", Line: 0}, bt[0])
	})

	t.Run("nested objects in vars", func(t *testing.T) {
		event := gjson.Parse(`{"exception":{"values":[{"stacktrace":{"frames":[{
			"filename":"test.js","lineno":10,
			"vars":{"params":{"foo":1,"bar":2,"second":{"glass":3}}}
		}]}}]}}`)
		bt := ComposeBacktrace(event)
		assert.Equal(t, []string{
			"params.foo=1",
			"params.bar=2",
			"params.second.glass=3",
		}, bt[0].Arguments)
	})

	t.Run("arrays in vars", func(t *testing.T) {
		event := gjson.Parse(`{"exception":{"values":[{"stacktrace":{"frames":[{
			"filename":"test.js","lineno":10,
			"vars":{"items":["first","second","third"]}
		}]}}]}}`)
		bt := ComposeBacktrace(event)
		assert.Equal(t, []string{"items.0=first", "items.1=second", "items.2=third"}, bt[0].Arguments)
	})

	t.Run("null values in vars", func(t *testing.T) {
		event := gjson.Parse(`{"exception":{"values":[{"stacktrace":{"frames":[{
			"filename":"test.js","lineno":10,
			"vars":{"nullValue":null,"normalValue":"test"}
		}]}}]}}`)
		bt := ComposeBacktrace(event)
		assert.Equal(t, []string{"nullValue=null", "normalValue=test"}, bt[0].Arguments)
	})

	t.Run("empty objects and arrays in vars", func(t *testing.T) {
		event := gjson.Parse(`{"exception":{"values":[{"stacktrace":{"frames":[{
			"filename":"test.js","lineno":10,
			"vars":{"emptyObject":{},"emptyArray":[],"normalValue":"test"}
		}]}}]}}`)
		bt := ComposeBacktrace(event)
		assert.Equal(t, []string{"emptyObject={}", "emptyArray=[]", "normalValue=test"}, bt[0].Arguments)
	})

	t.Run("reverses frames", func(t *testing.T) {
		event := gjson.Parse(`{"exception":{"values":[{"stacktrace":{"frames":[
			{"filename":"test1.js","lineno":10},
			{"filename":"test2.js","lineno":123}
		]}}]}}`)
		bt := ComposeBacktrace(event)
		assert.Equal(t, "test2.js", bt[0].File)
		assert.Equal(t, 123, bt[0].Line)
		assert.Equal(t, "test1.js", bt[1].File)
		assert.Equal(t, 10, bt[1].Line)
	})

	t.Run("trims to 20 newest frames", func(t *testing.T) {
		frames := make([]string, 0, 25)
		for i := 0; i < 25; i++ {
			frames = append(frames, fmt.Sprintf(`{"filename":"f%d.js","lineno":%d}`, i, i))
		}
		event := gjson.Parse(fmt.Sprintf(
			`{"exception":{"values":[{"stacktrace":{"frames":[%s]}}]}}`,
			strings.Join(frames, ","),
		))
		bt := ComposeBacktrace(event)
		require.Len(t, bt, 20)
		// After reverse, newest (f24) is first; oldest kept is f5
		assert.Equal(t, "f24.js", bt[0].File)
		assert.Equal(t, "f5.js", bt[19].File)
	})
}

func TestComposeContext(t *testing.T) {
	t.Run("returns contexts", func(t *testing.T) {
		event := gjson.Parse(`{"contexts":{"os":{"name":"Linux"}}}`)
		assert.JSONEq(t, `{"os":{"name":"Linux"}}`, string(ComposeContext(event)))
	})

	t.Run("nil when missing", func(t *testing.T) {
		assert.Nil(t, ComposeContext(gjson.Parse(`{}`)))
	})
}

func TestComposeAddons(t *testing.T) {
	t.Run("includes specified fields", func(t *testing.T) {
		event := gjson.Parse(`{
			"message":"Test message",
			"logentry":{"message":"Test log entry"},
			"timestamp":1718851200,
			"start_timestamp":1718851200,
			"level":"error",
			"platform":"javascript",
			"server_name":"test-server",
			"release":"1.0.0",
			"dist":"1.0.0",
			"environment":"production",
			"request":{"url":"https://test.com"},
			"transaction":"test-transaction",
			"modules":{"key":"value"},
			"fingerprint":["test-fingerprint"],
			"exception":{"values":[{"type":"Error","value":"Something went wrong"}]},
			"breadcrumbs":[{"message":"Test breadcrumb"}],
			"tags":{"key":"value"},
			"extra":{"key":"value"}
		}`)
		addons := ComposeAddons(event)
		assert.Equal(t, "Test message", addons["message"])
		assert.Equal(t, "error", addons["level"])
		assert.Equal(t, "javascript", addons["platform"])
		assert.NotContains(t, addons, "release")
		assert.NotContains(t, addons, "exception")
		assert.NotContains(t, addons, "breadcrumbs")
	})

	t.Run("excludes undefined fields", func(t *testing.T) {
		addons := ComposeAddons(gjson.Parse(`{"message":"Test message"}`))
		assert.Equal(t, map[string]interface{}{"message": "Test message"}, addons)
	})
}

func TestComposeUserData(t *testing.T) {
	t.Run("compose user", func(t *testing.T) {
		event := gjson.Parse(`{"user":{"id":"123","username":"testuser","email":"test@example.com"}}`)
		assert.Equal(t, &HawkUser{ID: "123", Name: "testuser", URL: "test@example.com"}, ComposeUserData(event))
	})

	t.Run("missing user", func(t *testing.T) {
		assert.Nil(t, ComposeUserData(gjson.Parse(`{}`)))
	})

	t.Run("missing user id", func(t *testing.T) {
		event := gjson.Parse(`{"user":{"username":"testuser","email":"test@example.com"}}`)
		assert.Equal(t, &HawkUser{ID: "unknown", Name: "testuser", URL: "test@example.com"}, ComposeUserData(event))
	})
}

func TestIsJsSDK(t *testing.T) {
	assert.True(t, IsJsSDK(gjson.Parse(`{"sdk":{"name":"browser"}}`)))
	assert.True(t, IsJsSDK(gjson.Parse(`{"sdk":{"name":"sentry.javascript.react"}}`)))
	assert.False(t, IsJsSDK(gjson.Parse(`{"sdk":{"name":"python"}}`)))
	assert.False(t, IsJsSDK(gjson.Parse(`{}`)))
}
