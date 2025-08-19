package queue

import "context"

type Consumer interface {
	Subscribe(ctx context.Context, topic string, handler func(context.Context, Message) error) error
}

type Producer interface {
	Publish(ctx context.Context, topic string, msg Message) error
}
