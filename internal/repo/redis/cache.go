package redisrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func New(addr, password string, db int, ttl time.Duration) *Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Cache{rdb: rdb, ttl: ttl}
}

func (c *Cache) Close() error { return c.rdb.Close() }

func tasksKey(teamID uint64, status string, assigneeID *uint64, page, size int) string {
	a := "any"
	if assigneeID != nil {
		a = fmt.Sprintf("%d", *assigneeID)
	}
	return fmt.Sprintf("tasks:team:%d:status:%s:assignee:%s:page:%d:size:%d",
		teamID, status, a, page, size)
}

func (c *Cache) GetTasks(ctx context.Context, teamID uint64, status string, assigneeID *uint64, page, size int, out any) (bool, error) {
	key := tasksKey(teamID, status, assigneeID, page, size)
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, out); err != nil {
		_ = c.rdb.Del(ctx, key).Err()
		return false, nil
	}
	return true, nil
}

func (c *Cache) SetTasks(ctx context.Context, teamID uint64, status string, assigneeID *uint64, page, size int, v any) error {
	key := tasksKey(teamID, status, assigneeID, page, size)
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, c.ttl).Err()
}

func (c *Cache) InvalidateTeamTasks(ctx context.Context, teamID uint64) error {
	prefix := fmt.Sprintf("tasks:team:%d:", teamID)
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
