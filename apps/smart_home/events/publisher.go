package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"smarthome/models"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	LegacySensorsExchange = "legacy.sensors"
	TelemetryRawExchange  = "telemetry.raw"
)

// LegacyHouseID is the house every monolith sensor belongs to in the new services:
// the monolith has no notion of houses.
const LegacyHouseID = "00000000-0000-0000-0000-000000000001"

// Publisher sends monolith events to RabbitMQ. Publishing is best-effort: if the broker is
// unavailable the error is logged and the HTTP request still succeeds.
type Publisher struct {
	url  string
	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewPublisher(url string) *Publisher {
	return &Publisher{url: url}
}

func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reset()
}

func (p *Publisher) reset() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.ch, p.conn = nil, nil
}

func (p *Publisher) channel() (*amqp.Channel, error) {
	if p.ch != nil && !p.ch.IsClosed() {
		return p.ch, nil
	}
	p.reset()
	conn, err := amqp.DialConfig(p.url, amqp.Config{Dial: amqp.DefaultDial(3 * time.Second)})
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	for _, ex := range []string{LegacySensorsExchange, TelemetryRawExchange} {
		if err := ch.ExchangeDeclare(ex, "topic", true, false, false, false, nil); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	p.conn, p.ch = conn, ch
	return ch, nil
}

func (p *Publisher) publish(exchange, routingKey, messageType string, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		log.Printf("events: marshal %s: %v", messageType, err)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ch, err := p.channel()
	if err != nil {
		log.Printf("events: RabbitMQ unavailable, %s not published: %v", messageType, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Type:         messageType,
		Body:         payload,
	})
	if err != nil {
		log.Printf("events: publish %s: %v", messageType, err)
		p.reset()
	}
}

// LegacySensorEvent tells Device Management Service about sensor changes in the monolith.
type LegacySensorEvent struct {
	EventType      string    `json:"eventType"`
	DeviceID       string    `json:"deviceId"`
	LegacySensorID int       `json:"legacySensorId"`
	HouseID        string    `json:"houseId"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Location       string    `json:"location"`
	Unit           string    `json:"unit"`
	OccurredAt     time.Time `json:"occurredAt"`
}

func (p *Publisher) PublishSensorEvent(eventType string, s models.Sensor) {
	if s.DeviceID == nil {
		return
	}
	p.publish(LegacySensorsExchange, "sensor."+eventType, "legacy.sensor."+eventType, LegacySensorEvent{
		EventType:      eventType,
		DeviceID:       *s.DeviceID,
		LegacySensorID: s.ID,
		HouseID:        LegacyHouseID,
		Name:           s.Name,
		Type:           string(s.Type),
		Location:       s.Location,
		Unit:           s.Unit,
		OccurredAt:     time.Now().UTC(),
	})
}

type reading struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
}

// TelemetryRaw follows the TelemetryRaw message of api/rabbitmq.asyncapi.yaml.
type TelemetryRaw struct {
	MessageID  string    `json:"messageId"`
	DeviceID   string    `json:"deviceId"`
	DeviceType string    `json:"deviceType"`
	HouseID    string    `json:"houseId"`
	Protocol   string    `json:"protocol"`
	Ts         time.Time `json:"ts"`
	ReceivedAt time.Time `json:"receivedAt"`
	Readings   []reading `json:"readings"`
}

func (p *Publisher) PublishTelemetry(s models.Sensor, value float64, unit string, ts time.Time) {
	if s.DeviceID == nil {
		return
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	deviceType := deviceTypeFor(s.Type)
	p.publish(TelemetryRawExchange, fmt.Sprintf("%s.%s", deviceType, *s.DeviceID), "telemetry.raw", TelemetryRaw{
		MessageID:  uuid.NewString(),
		DeviceID:   *s.DeviceID,
		DeviceType: deviceType,
		HouseID:    LegacyHouseID,
		Protocol:   "HTTP",
		Ts:         ts.UTC(),
		ReceivedAt: time.Now().UTC(),
		Readings:   []reading{{Metric: metricFor(s.Type), Value: value, Unit: unit}},
	})
}

// deviceTypeFor maps a monolith sensor type to a DeviceType code of Device Management Service.
func deviceTypeFor(t models.SensorType) string {
	if t == models.Temperature {
		return "temp_sensor"
	}
	return string(t)
}

func metricFor(t models.SensorType) string {
	if t == models.Temperature {
		return "temperature"
	}
	return string(t)
}
