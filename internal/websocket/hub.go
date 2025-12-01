package websocket

import (
	"context"
	"time"

	"github.com/centrifugal/centrifuge"
	"github.com/rs/zerolog/log"
)

// Hub manages the centrifuge node and channels
type Hub struct {
	node   *centrifuge.Node
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHub creates a new websocket hub using Centrifuge
func NewHub() *Hub {
	cfg := centrifuge.Config{
		LogLevel: centrifuge.LogLevelInfo,
		LogHandler: func(entry centrifuge.LogEntry) {
			// Map centrifuge log levels to zerolog
			switch entry.Level {
			case centrifuge.LogLevelTrace, centrifuge.LogLevelDebug:
				log.Debug().Str("component", "centrifuge").Msg(entry.Message)
			case centrifuge.LogLevelInfo:
				log.Info().Str("component", "centrifuge").Msg(entry.Message)
			case centrifuge.LogLevelWarn:
				log.Warn().Str("component", "centrifuge").Msg(entry.Message)
			case centrifuge.LogLevelError:
				log.Error().Str("component", "centrifuge").Msg(entry.Message)
			}
		},
	}

	node, err := centrifuge.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create centrifuge node")
	}

	return &Hub{
		node: node,
	}
}

// Start starts the hub
func (h *Hub) Start(ctx context.Context) {
	h.ctx, h.cancel = context.WithCancel(ctx)

	// Set up connection handler
	h.node.OnConnecting(func(ctx context.Context, event centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
		log.Debug().
			Str("client_id", event.ClientID).
			Str("transport", string(event.Transport.Name())).
			Msg("Client connecting")

		// Allow anonymous connections for now
		// You can add authentication here if needed
		return centrifuge.ConnectReply{
			Credentials: &centrifuge.Credentials{
				UserID: event.ClientID, // Use client ID as user ID for anonymous users
			},
		}, nil
	})

	h.node.OnConnect(func(client *centrifuge.Client) {
		log.Debug().
			Str("client_id", client.ID()).
			Str("user_id", client.UserID()).
			Msg("WebSocket client connected")

		// Set up client-level event handlers
		client.OnSubscribe(func(event centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
			log.Debug().
				Str("client_id", client.ID()).
				Str("channel", event.Channel).
				Msg("Client subscribing to channel")

			// Allow subscription to job_runs channel
			if event.Channel == "job_runs" {
				cb(centrifuge.SubscribeReply{}, nil)
				log.Debug().
					Str("client_id", client.ID()).
					Str("channel", event.Channel).
					Msg("Client subscribed successfully")
			} else {
				cb(centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied)
				log.Warn().
					Str("client_id", client.ID()).
					Str("channel", event.Channel).
					Msg("Client subscription denied")
			}
		})

		client.OnDisconnect(func(event centrifuge.DisconnectEvent) {
			log.Debug().
				Str("client_id", client.ID()).
				Str("reason", event.Reason).
				Msg("WebSocket client disconnected")
		})
	})

	// Run the node
	if err := h.node.Run(); err != nil {
		log.Fatal().Err(err).Msg("Failed to run centrifuge node")
	}

	// Give the node a moment to fully initialize its internal goroutines
	time.Sleep(100 * time.Millisecond)

	log.Info().Msg("WebSocket hub (Centrifuge) started and ready")
}

// Stop stops the hub
func (h *Hub) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	if h.node != nil {
		_ = h.node.Shutdown(context.Background())
	}
	log.Info().Msg("WebSocket hub (Centrifuge) stopped")
}

// Publish publishes a message to a channel
func (h *Hub) Publish(channel string, data []byte) error {
	_, err := h.node.Publish(channel, data)
	return err
}

// Node returns the underlying centrifuge node
func (h *Hub) Node() *centrifuge.Node {
	return h.node
}

// Broadcast sends an event to the job_runs channel
func (h *Hub) Broadcast(eventType EventType, payload map[string]interface{}) {
	event := Event{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	data, err := event.MarshalJSON()
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal event")
		return
	}

	if err := h.Publish("job_runs", data); err != nil {
		log.Error().Err(err).Str("event_type", string(eventType)).Msg("Failed to publish event")
	} else {
		log.Debug().
			Str("event_type", string(eventType)).
			Int("payload_size", len(data)).
			Int("subscribers", h.node.Hub().NumSubscribers("job_runs")).
			Int("clients", h.ClientCount()).
			Msg("Event broadcast to job_runs channel")
	}
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	return h.node.Hub().NumClients()
}
