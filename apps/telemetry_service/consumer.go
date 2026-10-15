package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	telemetryExchange = "telemetry.raw"
	telemetryQueue    = "telemetry.ingest"
)

// telemetryMessage is the TelemetryRaw message published by the monolith (and later by Device Gateway).
type telemetryMessage struct {
	DeviceID   string    `json:"deviceId"`
	DeviceType string    `json:"deviceType"`
	Ts         time.Time `json:"ts"`
	Readings   []struct {
		Metric string  `json:"metric"`
		Value  float64 `json:"value"`
		Unit   string  `json:"unit"`
	} `json:"readings"`
}

// Consumer reads raw telemetry from RabbitMQ and saves it to the store.
type Consumer struct {
	url   string
	store *Store
}

func NewConsumer(url string, store *Store) *Consumer {
	return &Consumer{url: url, store: store}
}

// Run consumes until ctx is cancelled, reconnecting if the broker goes away.
func (c *Consumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := c.consume(ctx); err != nil {
			log.Printf("consumer: %v, reconnecting in 5s", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Consumer) consume(ctx context.Context) error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(telemetryExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(telemetryQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(telemetryQueue, "#", telemetryExchange, false, nil); err != nil {
		return err
	}
	deliveries, err := ch.Consume(telemetryQueue, "telemetry-service", false, false, false, false, nil)
	if err != nil {
		return err
	}
	log.Printf("consumer: listening on queue %s", telemetryQueue)

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return amqp.ErrClosed
			}
			c.handle(ctx, d)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) {
	var msg telemetryMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil || msg.DeviceID == "" {
		log.Printf("consumer: dropping malformed message: %v", err)
		_ = d.Nack(false, false)
		return
	}

	readings := make([]Reading, 0, len(msg.Readings))
	for _, r := range msg.Readings {
		readings = append(readings, Reading{
			DeviceID:   msg.DeviceID,
			DeviceType: msg.DeviceType,
			Metric:     r.Metric,
			Value:      r.Value,
			Unit:       r.Unit,
			Ts:         msg.Ts,
		})
	}
	if err := c.store.Save(ctx, readings); err != nil {
		log.Printf("consumer: save failed, message requeued: %v", err)
		time.Sleep(time.Second)
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}
