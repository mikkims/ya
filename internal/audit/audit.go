package audit

import (
	"context"
	"log"
)

type Event struct {
	Timestamp int64  `json:"ts"`
	Action    string `json:"action"`
	UserID    string `json:"user_id"`
	URL       string `json:"url"`
}

type Observer interface {
	Observe(context.Context, Event) error
	Name() string
}

type Publisher struct{ observers []Observer }

func NewPublisher(observers ...Observer) *Publisher {
	return &Publisher{observers: append([]Observer(nil), observers...)}
}

func (p *Publisher) Publish(ctx context.Context, event Event) {
	if p == nil {
		return
	}
	for _, observer := range p.observers {
		if err := observer.Observe(ctx, event); err != nil {
			log.Printf("audit receiver %s failed", observer.Name())
		}
	}
}
