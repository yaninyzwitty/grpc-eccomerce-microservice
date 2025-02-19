package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/gocql/gocql"
)

// Message represents an event in the outbox table
type Message struct {
	ID        gocql.UUID
	EventType string
	Payload   string
	CreatedAt time.Time
}

// Product represents the product data
type Product struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Category    string    `json:"category"`
	Tags        []string  `json:"tags"`
	StockCount  int32     `json:"stock_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const getMessagesQuery = `
	SELECT id, event_type, payload, created_at FROM eccomerce_keyspace.products_outbox 
	WHERE bucket = ? ORDER BY id ASC LIMIT ?
`

// ProcessMessages fetches messages from Cassandra and sends them to Pulsar asynchronously
func ProcessMessages(ctx context.Context, session *gocql.Session, pulsarProducer pulsar.Producer, workerID, batchSize int) {
	slog.Info("Message worker started", "workerID", workerID)
	bucket := time.Now().Format("2006-01-02") // Current date as bucket key

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping message worker", "workerID", workerID)
			return
		default:
			messages, err := fetchMessages(ctx, session, bucket, batchSize)
			if err != nil {
				slog.Error("Error fetching messages", "workerID", workerID, "error", err)
				time.Sleep(2 * time.Second) // Prevent aggressive retries
				continue
			}

			if len(messages) == 0 {
				time.Sleep(2 * time.Second) // No messages, wait before retrying
				continue
			}

			// Process messages asynchronously
			for _, msg := range messages {
				go func(msg Message) {
					if err := sendToPulsar(ctx, pulsarProducer, msg, session, bucket); err != nil {
						slog.Error("Error processing message", "messageID", msg.ID, "error", err)
					}
				}(msg)
			}
		}
	}
}

// fetchMessages retrieves messages from the outbox in batches
func fetchMessages(ctx context.Context, session *gocql.Session, bucket string, batchSize int) ([]Message, error) {
	var messages []Message
	iter := session.Query(getMessagesQuery, bucket, batchSize).WithContext(ctx).Iter()

	var msg Message
	for iter.Scan(&msg.ID, &msg.EventType, &msg.Payload, &msg.CreatedAt) {
		messages = append(messages, msg)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return messages, nil
}

// sendToPulsar sends a message to Pulsar asynchronously and deletes it upon success
func sendToPulsar(ctx context.Context, producer pulsar.Producer, msg Message, session *gocql.Session, bucket string) error {
	var product Product
	if err := json.Unmarshal([]byte(msg.Payload), &product); err != nil {
		return fmt.Errorf("failed to unmarshal product data: %w", err)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Async send
	messageChan := make(chan error, 1)
	producer.SendAsync(ctx, &pulsar.ProducerMessage{
		Key:     fmt.Sprintf("%s:%v", msg.EventType, product.ID),
		Payload: payload,
	}, func(_ pulsar.MessageID, _ *pulsar.ProducerMessage, err error) {
		messageChan <- err
		close(messageChan)
	})

	select {
	case err := <-messageChan:
		if err != nil {
			return fmt.Errorf("failed to publish message: %w", err)
		}
	case <-ctx.Done():
		return fmt.Errorf("context canceled while publishing message")
	}

	// Delete message from outbox after successful send
	return deleteMessage(ctx, session, bucket, msg.ID)
}

// deleteMessage removes processed messages from Cassandra
func deleteMessage(ctx context.Context, session *gocql.Session, bucket string, msgID gocql.UUID) error {
	return session.Query(`
		DELETE FROM eccomerce_keyspace.products_outbox WHERE bucket = ? AND id = ?`,
		bucket, msgID,
	).WithContext(ctx).Exec()
}
