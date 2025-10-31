package rules

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

// Event 定义规则事件类型。
type Event string

const (
	// EventRulesChanged 当规则发生增删改时发布。
	EventRulesChanged Event = "rules_changed"
)

// EventBus 用于广播和订阅规则变更。
type EventBus interface {
	Publish(ctx context.Context, evt Event) error
	Subscribe(ctx context.Context) (<-chan Event, error)
}

// RedisEventBus 基于 Redis Pub/Sub 的事件总线。
type RedisEventBus struct {
	client  *redis.Client
	channel string
}

// NewRedisEventBus 创建事件总线。
func NewRedisEventBus(client *redis.Client, channel string) *RedisEventBus {
	return &RedisEventBus{client: client, channel: channel}
}

func (b *RedisEventBus) Publish(ctx context.Context, evt Event) error {
	return b.client.Publish(ctx, b.channel, string(evt)).Err()
}

func (b *RedisEventBus) Subscribe(ctx context.Context) (<-chan Event, error) {
	sub := b.client.Subscribe(ctx, b.channel)
	if _, err := sub.Receive(ctx); err != nil {
		return nil, err
	}
	ch := make(chan Event)
	go func() {
		defer close(ch)
		for {
			msg, err := sub.ReceiveMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				// 订阅失败时退出，由调用方决定是否重试。
				return
			}
			ch <- Event(msg.Payload)
		}
	}()
	return ch, nil
}
