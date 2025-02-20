package helpers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/gocql/gocql"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProductEvent struct {
	ID        string    `json:"ID"`
	EventType string    `json:"EventType"`
	Payload   string    `json:"Payload"`
	CreatedAt time.Time `json:"CreatedAt"`
}

type ProductPayload struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Price       float64                `json:"price"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags"`
	CreatedAt   *timestamppb.Timestamp `json:"created_at"`
	UpdatedAt   *timestamppb.Timestamp `json:"updated_at"`
}

func ConsumeMessages(ctx context.Context, consumer pulsar.Consumer, session *gocql.Session) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down message consumer")
			return
		default:
			msg, err := consumer.Receive(ctx)
			if err != nil {
				slog.Error("Failed to receive message", "error", err)
				continue
			}

			var message ProductEvent
			if err := json.Unmarshal(msg.Payload(), &message); err != nil {
				slog.With("message_id", msg.ID()).Error("Failed to unmarshal message", "error", err)
				consumer.Nack(msg)
				continue
			}

			var product ProductPayload
			if err := json.Unmarshal([]byte(message.Payload), &product); err != nil {
				slog.With("message_id", msg.ID()).Error("Failed to unmarshal product", "error", err)
				consumer.Nack(msg)
				continue
			}

			goTime := product.CreatedAt.AsTime()

			if message.EventType == "CREATE_PRODUCT_EVENT" {
				slog.With("product_id", product.ID).Info("Received inventory message")

				if err := SaveInventory(ctx, session, product.ID, product.Category, 0, goTime); err != nil {
					slog.With("product_id", product.ID).Error("Failed to save inventory", "error", err)
					consumer.Nack(msg)
					continue
				}
			}

			consumer.Ack(msg)
		}
	}
}

func SaveInventory(ctx context.Context, session *gocql.Session, productID int64, category string, stockCount int32, createdAt time.Time) error {
	query := `INSERT INTO eccomerce_keyspace.inventory (product_id, category, stock_count, created_at, last_updated_at) VALUES (?, ?, ?, ?, ?)`
	err := session.Query(query, productID, category, stockCount, createdAt, time.Now()).WithContext(ctx).Exec()
	if err != nil {
		slog.With("product_id", productID).Error("Failed to save inventory", "error", err)
		return err
	}
	return nil
}
