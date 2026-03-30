package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database/sheets"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type SheetCache interface {
	Get(ctx context.Context, key string, sheetRange string, ttl time.Duration) ([][]interface{}, error)
	Invalidate(ctx context.Context, key string) error
	StartRefresher(ctx context.Context)
}

type sheetsCache struct {
	rdb          *redis.Client
	sheetsClient *sheets.SheetsClient
	logger       zerolog.Logger
}

func NewSheetCache(rdb *redis.Client, sheetsClient *sheets.SheetsClient, logger zerolog.Logger) SheetCache {
	return &sheetsCache{
		rdb:          rdb,
		sheetsClient: sheetsClient,
		logger:       logger,
	}
}

func (c *sheetsCache) Get(ctx context.Context, key string, sheetRange string, ttl time.Duration) ([][]interface{}, error) {
	cached, err := c.rdb.Get(ctx, key).Result()
	if err == nil {
		var rows [][]interface{}
		if jsonErr := json.Unmarshal([]byte(cached), &rows); jsonErr == nil {
			return rows, nil
		}
	}

	rows, err := c.sheetsClient.ReadRange(ctx, sheetRange)
	if err != nil {
		return nil, fmt.Errorf("cache miss fetch failed for %s: %w", key, err)
	}

	data, err := json.Marshal(rows)
	if err != nil {
		c.logger.Warn().Err(err).Str("key", key).Msg("Failed to marshal rows for cache")
		return rows, nil
	}

	if setErr := c.rdb.Set(ctx, key, data, ttl).Err(); setErr != nil {
		c.logger.Warn().Err(setErr).Str("key", key).Msg("Failed to store in Redis cache")
	}

	return rows, nil
}

func (c *sheetsCache) Invalidate(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *sheetsCache) StartRefresher(ctx context.Context) {
	refreshTargets := []struct {
		key        string
		sheetRange string
		ttl        time.Duration
	}{
		{KeyTriggerTemplate, RangeTriggerTemplate, TTLTriggerTemplate},
		{KeyMasterClient, RangeMasterClient, TTLMasterClient},
		{KeyInvoices, RangeInvoices, TTLInvoices},
		{KeyConversationState, RangeConversationState, TTLConversationState},
		{KeyEscalation, RangeEscalation, TTLEscalation},
	}

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		c.logger.Info().Msg("Sheet cache background refresher started")

		for {
			select {
			case <-ctx.Done():
				c.logger.Info().Msg("Sheet cache background refresher stopped")
				return
			case <-ticker.C:
				for _, t := range refreshTargets {
					rows, err := c.sheetsClient.ReadRange(ctx, t.sheetRange)
					if err != nil {
						c.logger.Warn().Err(err).Str("key", t.key).Msg("Background refresh failed")
						continue
					}

					data, err := json.Marshal(rows)
					if err != nil {
						c.logger.Warn().Err(err).Str("key", t.key).Msg("Background refresh marshal failed")
						continue
					}

					if setErr := c.rdb.Set(ctx, t.key, data, t.ttl).Err(); setErr != nil {
						c.logger.Warn().Err(setErr).Str("key", t.key).Msg("Background refresh set failed")
					}
				}
			}
		}
	}()
}
