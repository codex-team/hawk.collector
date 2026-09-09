package errorshandler

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/codex-team/hawk.collector/pkg/accounts"
	"github.com/codex-team/hawk.collector/pkg/broker"
	"github.com/codex-team/hawk.collector/pkg/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// TestHandleSentry_OneC_BareJSONBody drives the real HTTP handler end-to-end
// (token lookup, rate limiting, envelope parsing, broker publish) with the
// exact bare-JSON body 1C sent, to confirm the ParseEnvelope/ProcessEnvelope
// fix actually reaches the broker queue — not just the pure parsing layer
// covered by sentry.TestProcessEnvelope_OneC_BareJSONBody.
func TestHandleSentry_OneC_BareJSONBody(t *testing.T) {
	const projectID = "68c05fabc1a2b3c4d5e6f708"
	const hawkToken = "test-hawk-token"

	// Real (in-memory) Redis so RedisClient.recordProjectMetrics' TS.ADD calls
	// hit an actual connection instead of panicking on a nil *redis.Client —
	// IsBlocked/UpdateRateLimit don't need it (see below) but this keeps the
	// handler's full code path exercised, same as production.
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	handler := &Handler{
		Broker:                        &broker.Broker{Chan: make(chan broker.Message, 1)},
		MaxErrorCatcherMessageSize:    10 * 1024 * 1024,
		ErrorsBlockedByLimit:          prometheus.NewCounter(prometheus.CounterOpts{Name: "test_errors_blocked"}),
		ErrorsProcessed:               prometheus.NewCounter(prometheus.CounterOpts{Name: "test_errors_processed"}),
		ErrorsRejectedMessageTooLarge: prometheus.NewCounter(prometheus.CounterOpts{Name: "test_errors_rejected"}),
		RedisClient:                   redis.New(context.Background(), mr.Addr(), "", "blocked-ids", "blacklist-ips", "all-ips", "current-period"),
		AccountsMongoDBClient:         accounts.NewWithTokenCache(map[string]string{hawkToken: projectID}),
	}
	// No project limits registered for projectID -> GetProjectLimits reports
	// !ok and UpdateRateLimit short-circuits (EventsLimit 0 means "no limit"),
	// so the request isn't rate-limited.

	body := `{"event_id":"9961317b-5d63-409f-b231-f65b6657ae5b","timestamp":"2026-09-07T12:33:09","level":"error","logger":"1C","platform":"other","message":"Ошибка при вызове метода контекста (Получить)\n{ВнешняяОбработка.Сентри.Форма.Форма.Форма(259)}:Возврат Константы.Sentry_ДатаИВремяПоследнейОтправкиОшибки.Получить() + 1;\n{ВнешняяОбработка.Сентри.Форма.Форма.Форма(7)}:ДатаПоследнейОтправки = ПолучитьПоследнююДатуОтправки();\n{ВнешняяОбработка.Сентри.Форма.Форма.Форма(268)}:ОтправитьОшибкиИзЖурналаРегистрации();\n\n[ОшибкаВоВремяВыполненияВстроенногоЯзыка]\nпо причине:\nНарушение прав доступа!\n[НарушениеПравДоступа]","extra":{"application":"1CV8C","user":"2b85e012-b40b-4b57-bbe0-8996846d12c5","event":"_$PerformError$_","data":"","comment":"Ошибка при вызове метода контекста (Получить)\n{ВнешняяОбработка.Сентри.Форма.Форма.Форма(259)}:Возврат Константы.Sentry_ДатаИВремяПоследнейОтправкиОшибки.Получить() + 1;\n{ВнешняяОбработка.Сентри.Форма.Форма.Форма(7)}:ДатаПоследнейОтправки = ПолучитьПоследнююДатуОтправки();\n{ВнешняяОбработка.Сентри.Форма.Форма.Форма(268)}:ОтправитьОшибкиИзЖурналаРегистрации();\n\n[ОшибкаВоВремяВыполненияВстроенногоЯзыка]\nпо причине:\nНарушение прав доступа!\n[НарушениеПравДоступа]"},"tags":{"source":"journal"}}`

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/0/envelope/?sentry_key=" + hawkToken)
	ctx.Request.SetBody([]byte(body))

	handler.HandleSentry(&ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	select {
	case msg := <-handler.Broker.Chan:
		assert.Equal(t, "errors/default", msg.Route)
		assert.Contains(t, string(msg.Payload), projectID)
		assert.Contains(t, string(msg.Payload), "Ошибка при вызове метода контекста")
	default:
		t.Fatal("expected HandleSentry to publish a message to the broker, got none")
	}

	require.Equal(t, float64(1), testutil.ToFloat64(handler.ErrorsProcessed))
}
