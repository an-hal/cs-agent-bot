package session

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type redisSessionStore struct {
	rdb    *redis.Client
	logger zerolog.Logger
}

func NewRedisSessionStore(rdb *redis.Client, logger zerolog.Logger) Store {
	return &redisSessionStore{
		rdb:    rdb,
		logger: logger,
	}
}

func (s *redisSessionStore) GetUserIDByToken(ctx context.Context, token string) (string, error) {
	key := "session:" + token
	userID, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return userID, nil
}

func (s *redisSessionStore) GetSession(ctx context.Context, sid string) (Session, error) {
	var sess Session

	redisKey := "active_session_access_" + sid

	result, err := s.rdb.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		return Session{}, err
	}

	if err != nil {
		return sess, err
	}

	err = json.Unmarshal([]byte(result), &sess)

	return sess, err
}
