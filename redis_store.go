package salvador

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements Store using Redis for task persistence and queuing.
type RedisStore struct {
	client    *redis.Client
	keyPrefix string
}

// RedisOption configures the RedisStore.
type RedisOption func(*RedisStore)

// WithKeyPrefix sets a prefix for all Redis keys. Defaults to "salvador:".
func WithKeyPrefix(prefix string) RedisOption {
	return func(s *RedisStore) {
		s.keyPrefix = prefix
	}
}

// NewRedisStore creates a new Redis-backed store.
func NewRedisStore(client *redis.Client, opts ...RedisOption) *RedisStore {
	s := &RedisStore{
		client:    client,
		keyPrefix: "salvador:",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *RedisStore) taskKey(id string) string {
	return fmt.Sprintf("%stask:%s", s.keyPrefix, id)
}

func (s *RedisStore) queueKey(workflowName string) string {
	return fmt.Sprintf("%squeue:%s", s.keyPrefix, workflowName)
}

func (s *RedisStore) Save(ctx context.Context, task *Task) error {
	task.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	return s.client.Set(ctx, s.taskKey(task.ID), data, 0).Err()
}

func (s *RedisStore) Get(ctx context.Context, id string) (*Task, error) {
	data, err := s.client.Get(ctx, s.taskKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("task %q not found", id)
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}
	return &task, nil
}

func (s *RedisStore) Enqueue(ctx context.Context, workflowName string, taskID string) error {
	return s.client.LPush(ctx, s.queueKey(workflowName), taskID).Err()
}

func (s *RedisStore) Dequeue(ctx context.Context, workflowName string) (string, error) {
	val, err := s.client.RPop(ctx, s.queueKey(workflowName)).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("dequeue: %w", err)
	}
	return val, nil
}
