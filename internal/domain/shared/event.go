package shared

import "time"

type Event interface {
	EventID() string
	EventType() string
	OcurredAt() time.Time
	AggregateID() string
}

type BaseEvent struct {
	ID        string    `json:"event_id"`
	Type      string    `json:"event_type"`
	At        time.Time `json:"occurred_at"`
	Aggregate string    `json:"aggregate_id"`
	Version   int       `json:"version"`
}

func NewBaseEvent(eventType, aggregateID string, at time.Time) BaseEvent {
	return BaseEvent{ID: NewID("evt"), Type: eventType, At: at, Aggregate: aggregateID, Version: 1}
}

func (b BaseEvent) EventID() string       { return b.ID }
func (b BaseEvent) EventType() string     { return b.Type }
func (b BaseEvent) OccurredAt() time.Time { return b.At }
func (b BaseEvent) AggregateID() string   { return b.Aggregate }
