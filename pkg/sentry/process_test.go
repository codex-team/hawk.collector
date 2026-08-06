package sentry

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessEnvelope_MultipleEvents(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"2024-01-01T00:00:00.000Z"}`,
		`{"type":"event","content_type":"application/json"}`,
		`{"level":"error","message":"Error 1"}`,
		`{"type":"event","content_type":"application/json"}`,
		`{"level":"warning","message":"Warning 1"}`,
	)

	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, "error", messages[0].Payload.Type)
	assert.Equal(t, "warning", messages[1].Payload.Type)
	assert.Equal(t, WorkerTypeDefault, messages[0].CatcherType)
}

func TestProcessEnvelope_EmptyItems(t *testing.T) {
	envelope := `{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"2024-01-01T00:00:00.000Z"}`
	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestProcessEnvelope_SkipsNonEventItems(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"2024-01-01T00:00:00.000Z"}`,
		`{"type":"transaction"}`,
		`{"name":"Test Transaction"}`,
		`{"type":"attachment"}`,
		`{"filename":"test.txt"}`,
		`{"type":"client_report"}`,
		`{"timestamp":1718534400,"discarded_events":[]}`,
	)

	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestProcessEnvelope_MissingPayload(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"2024-01-01T00:00:00.000Z"}`,
		`{"type":"event"}`,
		`null`,
	)

	_, err := ProcessEnvelope([]byte(envelope), "123")
	require.Error(t, err)
}

func TestProcessEnvelope_TimestampFormats(t *testing.T) {
	timestamps := []string{
		"2024-01-01T00:00:00.000Z",
		"2024-01-01T00:00:00Z",
		"2024-01-01T00:00:00.000+00:00",
	}

	for _, ts := range timestamps {
		envelope := joinLines(
			`{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"`+ts+`"}`,
			`{"type":"event"}`,
			`{"message":"Test timestamp"}`,
		)
		messages, err := ProcessEnvelope([]byte(envelope), "123")
		require.NoError(t, err, ts)
		require.Len(t, messages, 1)
		assert.Equal(t, int64(1704067200), messages[0].Timestamp, ts)
		assert.Equal(t, "Test timestamp", messages[0].Payload.Title)
		assert.Equal(t, CatcherVersion, messages[0].Payload.CatcherVersion)
	}
}

func TestProcessEnvelope_RoutesJsSDK(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"2024-01-01T00:00:00.000Z"}`,
		`{"type":"event"}`,
		`{"sdk":{"name":"browser"},"release":"1.0.0"}`,
	)
	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, WorkerTypeJavaScript, messages[0].CatcherType)
	assert.Equal(t, "1.0.0", messages[0].Payload.Release)
}

func TestProcessEnvelope_RoutesNonJsSDK(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"2024-01-01T00:00:00.000Z"}`,
		`{"type":"event"}`,
		`{"sdk":{"name":"python"},"release":"1.0.0"}`,
	)
	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, WorkerTypeDefault, messages[0].CatcherType)
}

func TestProcessEnvelope_ReleasePreference(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"123","sent_at":"2024-01-01T00:00:00.000Z","trace":{"release":"1.0.0"}}`,
		`{"type":"event"}`,
		`{"release":"1.0.1"}`,
	)
	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	assert.Equal(t, "1.0.1", messages[0].Payload.Release)
}

func TestProcessEnvelope_ReleaseFromTrace(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"123","sent_at":"2024-01-01T00:00:00.000Z","trace":{"release":"1.0.0"}}`,
		`{"type":"event"}`,
		`{"message":"Test"}`,
	)
	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", messages[0].Payload.Release)
}

func TestProcessEnvelope_FiltersReplayOnly(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"62680958b3ab4497886375e06533d86a","sent_at":"2025-12-24T13:16:34.580Z","sdk":{"name":"sentry.javascript.react"}}`,
		`{"type":"replay_event"}`,
		`{"type":"replay_event","replay_id":"62680958b3ab4497886375e06533d86a","segment_id":1}`,
		`{"type":"replay_recording","length":16385}`,
		`{"segment_id":1}`,
		`xnFWy@v$xAlJ=&fS~binary`,
	)
	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestProcessEnvelope_MixedEventAndReplay(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"4c40fee730194a989439a86bf75634111","sent_at":"2025-08-29T10:59:29.952Z","sdk":{"name":"sentry.javascript.react"}}`,
		`{"type":"event"}`,
		`{"message":"Test event","level":"error"}`,
		`{"type":"replay_event"}`,
		`{"replay_id":"test-replay","segment_id":1}`,
		`{"type":"replay_recording","length":343}`,
		`binary-data-here-that-is-not-json`,
	)
	messages, err := ProcessEnvelope([]byte(envelope), "621601f4a010d35c68b4625a")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	// SDK is only on envelope header; Node routes by item payload.sdk → default
	assert.Equal(t, WorkerTypeDefault, messages[0].CatcherType)
	require.NotNil(t, messages[0].Payload.Addons)
	sentryAddons := messages[0].Payload.Addons["sentry"].(map[string]interface{})
	assert.Equal(t, "Test event", sentryAddons["message"])
	assert.Equal(t, "error", sentryAddons["level"])
}

func TestProcessEnvelope_RealPythonError(t *testing.T) {
	b64 := "eyJldmVudF9pZCI6ImE3OWVhYjNmY2ZjMDQ1ZjM5MTUyNzM5NWJmYzhlZGM2Iiwic2VudF9hdCI6IjIwMjQtMTItMThUMTQ6MDA6MTIuODY2NjYwWiIsInRyYWNlIjp7InRyYWNlX2lkIjoiYTAwMDA1NzEwOGFlNDJhMGIxNTRhY2ZmN2U5YTUxMTQiLCJlbnZpcm9ubWVudCI6InByb2R1Y3Rpb24iLCJyZWxlYXNlIjoiMzVjNDJmMmRkZjUyNGJiZGFmZTIyNDM5Yjk4MWRkZmQwZmIzYjVhZCIsInB1YmxpY19rZXkiOiIyMGRlYTRjN2NmZjg0NWZlYmE3YWJmNDlhYzZhOTdlZWFiMTg4NDNhOTczNDRmODZhY2QzNGM3MzAxNDc3YTQxIn19CnsidHlwZSI6ImV2ZW50IiwiY29udGVudF90eXBlIjoiYXBwbGljYXRpb24vanNvbiIsImxlbmd0aCI6MTk5MH0KeyJsZXZlbCI6ImVycm9yIiwiZXhjZXB0aW9uIjp7InZhbHVlcyI6W3sibWVjaGFuaXNtIjp7InR5cGUiOiJleGNlcHRob29rIiwiaGFuZGxlZCI6ZmFsc2V9LCJtb2R1bGUiOm51bGwsInR5cGUiOiJaZXJvRGl2aXNpb25FcnJvciIsInZhbHVlIjoiZGl2aXNpb24gYnkgemVybyIsInN0YWNrdHJhY2UiOnsiZnJhbWVzIjpbeyJmaWxlbmFtZSI6InNlbnRyeS1wcm9kLnB5IiwiYWJzX3BhdGgiOiIvVXNlcnMvbm9zdHIvZGV2L2NvZGV4L2hhd2subW9uby90ZXN0cy9tYW51YWwvc2VudHJ5L3NlbnRyeS1wcm9kLnB5IiwiZnVuY3Rpb24iOiI8bW9kdWxlPiIsIm1vZHVsZSI6Il9fbWFpbl9fIiwibGluZW5vIjoxMSwicHJlX2NvbnRleHQiOlsiIiwic2VudHJ5X3Nkay5pbml0KCIsIiAgICBkc249ZlwiaHR0cHM6Ly97SEFXS19JTlRFR1JBVElPTl9UT0tFTn1Ae0hPU1R9LzBcIiwiLCIgICAgZGVidWc9VHJ1ZSIsIikiXSwiY29udGV4dF9saW5lIjoiZGl2aXNpb25fYnlfemVybyA9IDEgLyAwIiwicG9zdF9jb250ZXh0IjpbInByaW50KFwidGhpc1wiKSIsInByaW50KFwiaXNcIikiLCJwcmludChcIm9rXCIpIiwiIyByYWlzZSBFeGNlcHRpb24oXCJUaGlzIGlzIGEgdGVzdCBleGNlcHRpb25cIikiXSwidmFycyI6eyJfX25hbWVfXyI6IidfX21haW5fXyciLCJfX2RvY19fIjoiTm9uZSIsIl9fcGFja2FnZV9fIjoiTm9uZSIsIl9fbG9hZGVyX18iOiI8X2Zyb3plbl9pbXBvcnRsaWJfZXh0ZXJuYWwuU291cmNlRmlsZUxvYWRlciBvYmplY3QgYXQgMHgxMDI5MzRjYjA+IiwiX19zcGVjX18iOiJOb25lIiwiX19hbm5vdGF0aW9uc19fIjp7fSwiX19idWlsdGluc19fIjoiPG1vZHVsZSAnYnVpbHRpbnMnIChidWlsdC1pbik+IiwiX19maWxlX18iOiInL1VzZXJzL25vc3RyL2Rldi9jb2RleC9oYXdrLm1vbm8vdGVzdHMvbWFudWFsL3NlbnRyeS9zZW50cnktcHJvZC5weSciLCJfX2NhY2hlZF9fIjoiTm9uZSIsInNlbnRyeV9zZGsiOiI8bW9kdWxlICdzZW50cnlfc2RrJyBmcm9tICcvVXNlcnMvbm9zdHIvZGV2L2NvZGV4L2hhd2subW9uby8udmVudi9saWIvcHl0aG9uMy4xMy9zaXRlLXBhY2thZ2VzL3NlbnRyeV9zZGsvX19pbml0X18ucHknPiJ9LCJpbl9hcHAiOnRydWV9XX19XX0sImV2ZW50X2lkIjoiYTc5ZWFiM2ZjZmMwNDVmMzkxNTI3Mzk1YmZjOGVkYzYiLCJ0aW1lc3RhbXAiOiIyMDI0LTEyLTE4VDE0OjAwOjEyLjg2MzY4NFoiLCJjb250ZXh0cyI6eyJ0cmFjZSI6eyJ0cmFjZV9pZCI6ImEwMDAwNTcxMDhhZTQyYTBiMTU0YWNmZjdlOWE1MTE0Iiwic3Bhbl9pZCI6ImFmNzI2MjMwODk4ODJiNjciLCJwYXJlbnRfc3Bhbl9pZCI6bnVsbH0sInJ1bnRpbWUiOnsibmFtZSI6IkNQeXRob24iLCJ2ZXJzaW9uIjoiMy4xMy4xIiwiYnVpbGQiOiIzLjEzLjEgKG1haW4sIERlYyAgMyAyMDI0LCAxNzo1OTo1MikgW0NsYW5nIDE2LjAuMCAoY2xhbmctMTYwMC4wLjI2LjQpXSJ9fSwidHJhbnNhY3Rpb25faW5mbyI6e30sImJyZWFkY3J1bWJzIjp7InZhbHVlcyI6W119LCJleHRyYSI6eyJzeXMuYXJndiI6WyJzZW50cnktcHJvZC5weSJdfSwibW9kdWxlcyI6eyJwaXAiOiIyNC4zLjEiLCJ1cmxsaWIzIjoiMi4yLjMiLCJzZW50cnktc2RrIjoiMi4xOS4wIiwiY2VydGlmaSI6IjIwMjQuOC4zMCJ9LCJyZWxlYXNlIjoiMzVjNDJmMmRkZjUyNGJiZGFmZTIyNDM5Yjk4MWRkZmQwZmIzYjVhZCIsImVudmlyb25tZW50IjoicHJvZHVjdGlvbiIsInNlcnZlcl9uYW1lIjoiTWFjQm9vay1Qcm8tQWxla3NhbmRyLTUubG9jYWwiLCJzZGsiOnsibmFtZSI6InNlbnRyeS5weXRob24iLCJ2ZXJzaW9uIjoiMi4xOS4wIiwicGFja2FnZXMiOlt7Im5hbWUiOiJweXBpOnNlbnRyeS1zZGsiLCJ2ZXJzaW9uIjoiMi4xOS4wIn1dLCJpbnRlZ3JhdGlvbnMiOlsiYXJndiIsImF0ZXhpdCIsImRlZHVwZSIsImV4Y2VwdGhvb2siLCJsb2dnaW5nIiwibW9kdWxlcyIsInN0ZGxpYiIsInRocmVhZGluZyJdfSwicGxhdGZvcm0iOiJweXRob24ifQo="
	raw, err := base64.StdEncoding.DecodeString(b64)
	require.NoError(t, err)

	messages, err := ProcessEnvelope(raw, "675c9605b8264d74b5a7dcf3")
	require.NoError(t, err)
	require.Len(t, messages, 1)

	msg := messages[0]
	assert.Equal(t, int64(1734530412), msg.Timestamp)
	assert.Equal(t, WorkerTypeDefault, msg.CatcherType)
	assert.Equal(t, "ZeroDivisionError: division by zero", msg.Payload.Title)
	assert.Equal(t, "error", msg.Payload.Type)
	assert.Equal(t, "35c42f2ddf524bbdafe22439b981ddfd0fb3b5ad", msg.Payload.Release)
	assert.Equal(t, CatcherVersion, msg.Payload.CatcherVersion)
	require.Len(t, msg.Payload.Backtrace, 1)
	assert.Equal(t, "sentry-prod.py", msg.Payload.Backtrace[0].File)
	assert.Equal(t, 11, msg.Payload.Backtrace[0].Line)
	assert.Equal(t, "<module>", msg.Payload.Backtrace[0].Function)
}

func TestProcessEnvelope_CyrillicTitle(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"4b6140fb97504945908fe52c5b0dd123","sent_at":"2025-04-03T13:27:27.348Z"}`,
		`{"type":"event","content_type":"application/json"}`,
		`{"exception":{"values":[{"type":"Exception","value":"Тестовая ошибка #287"}]},"sdk":{"name":"sentry.java.android"}}`,
	)
	messages, err := ProcessEnvelope([]byte(envelope), "67ed371b4196dcbd73537c64")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "Exception: Тестовая ошибка #287", messages[0].Payload.Title)
}

func TestProcessEnvelope_JSONPayloadWithoutLength(t *testing.T) {
	envelope := joinLines(
		`{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"2024-01-01T00:00:00.000Z"}`,
		`{"type":"event","content_type":"application/json"}`,
		`{"message":"plain json payload","level":"error"}`,
	)
	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "plain json payload", messages[0].Payload.Title)
	assert.Equal(t, "error", messages[0].Payload.Type)
}

func TestProcessEnvelope_LengthPrefixedPayload(t *testing.T) {
	// Pretty-printed JSON contains real newlines; only length-aware parsing reads it correctly.
	payload := "{\n  \"message\": \"pretty json payload\",\n  \"level\": \"error\"\n}"
	header := fmt.Sprintf(`{"type":"event","content_type":"application/json","length":%d}`, len(payload))
	envelope := joinLines(
		`{"event_id":"123e4567-e89b-12d3-a456-426614174000","sent_at":"2024-01-01T00:00:00.000Z"}`,
		header,
	) + "\n" + payload + "\n"

	messages, err := ProcessEnvelope([]byte(envelope), "123")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "pretty json payload", messages[0].Payload.Title)
	assert.Equal(t, "error", messages[0].Payload.Type)
}

func joinLines(lines ...string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}
