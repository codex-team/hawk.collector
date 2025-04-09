package errorshandler

import (
	"encoding/json"

	"github.com/codex-team/hawk.collector/pkg/hawk"
	"github.com/fasthttp/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

// WebSocket metrics
var (
	// Current active WebSocket connections
	websocketActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "websocket_connections_active",
		Help: "Number of currently active WebSocket connections",
	})

	// Total connections established
	websocketConnectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "websocket_connections_total",
		Help: "Total number of WebSocket connections established",
	})

	// Messages received
	websocketMessagesReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "websocket_messages_received_total",
		Help: "Total number of WebSocket messages received",
	})

	// Messages sent
	websocketMessagesSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "websocket_messages_sent_total",
		Help: "Total number of WebSocket messages sent",
	})

	// Message processing errors (non-connection errors)
	websocketMessageErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "websocket_message_errors_total",
		Help: "Total number of WebSocket message processing errors",
	})

	// Connection errors 
	websocketConnectionErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "websocket_connection_errors_total",
		Help: "Total number of WebSocket connection errors",
	})
)

// WebSocket upgrader options
var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *fasthttp.RequestCtx) bool {
		return true
	},
}

// HandleWebsocket handles WebSocket connection
func (handler *Handler) HandleWebsocket(ctx *fasthttp.RequestCtx) {
	// Increment connection counter
	websocketConnectionsTotal.Inc()
	
	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		// Increment active connections gauge
		websocketActiveConnections.Inc()
		// Ensure we decrement the gauge when connection ends
		defer websocketActiveConnections.Dec()
		
		// limit read size of MaxErrorCatcherMessageSize bytes
		conn.SetReadLimit(int64(handler.MaxErrorCatcherMessageSize))
		
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				websocketConnectionErrors.Inc()
				log.Errorf("Websocket error in ReadMessage: %v", err)
				break
			}

			// Increment messages received counter
			websocketMessagesReceived.Inc()
			
			log.Debugf("Websocket message: %s", message)

			// process raw body via unified message handler
			response := handler.process(message)
			log.Debugf("Websocket response: %s", response.Message)

			if err = sendAnswerWebsocket(conn, messageType, response); err != nil {
				websocketMessageErrors.Inc()
				log.Errorf("Websocket response: %v", err)
				return
			}
			
			// Increment messages sent counter
			websocketMessagesSent.Inc()
		}
	})

	// log if connection is closed ungracefully
	if err != nil {
		websocketConnectionErrors.Inc()
		// Do not catch WebSocket upgrade erros, since it's usually client malformed requests
		if _, ok := err.(websocket.HandshakeError); !ok {
			hawk.Catch(err)
		}
		log.Errorf("Websocket error: %v", err)
	}
}

// Send ResponseMessage in JSON
func sendAnswerWebsocket(conn *websocket.Conn, messageType int, r ResponseMessage) error {
	response, err := json.Marshal(r)
	if err != nil {
		hawk.Catch(err)
		return err
	}

	return conn.WriteMessage(messageType, response)
}
