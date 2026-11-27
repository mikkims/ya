package audit

import (
	"context"
	"log"
)

// Event описывает событие аудита.
type Event struct {
	// Timestamp время события в секундах Unix.
	Timestamp int64 `json:"ts"`
	// Action задаёт действие: shorten для создания URL, follow для перехода.
	Action string `json:"action"`
	// UserID идентификатор пользователя.
	UserID string `json:"user_id"`
	// URL исходный URL.
	URL string `json:"url"`
}

// Observer принимает события аудита.
type Observer interface {
	// Observe передаёт событие получателю с указанным контекстом.
	Observe(context.Context, Event) error
	// Name возвращает имя получателя.
	Name() string
}

// Publisher передаёт события наблюдателям.
type Publisher struct{ observers []Observer }

// NewPublisher создаёт издателя с копией списка наблюдателей.
func NewPublisher(observers ...Observer) *Publisher {
	return &Publisher{observers: append([]Observer(nil), observers...)}
}

// Publish последовательно передаёт событие наблюдателям и журналирует ошибки доставки.
// Ошибка одного наблюдателя не прерывает остальные; вызов на nil *Publisher допустим.
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
