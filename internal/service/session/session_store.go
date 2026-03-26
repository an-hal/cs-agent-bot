package session

import (
	"context"
)

type Session struct {
	HalfSecret string `json:"hs"` // half piece of secret which stored in session db (the other half is from config)
}

type Store interface {
	GetUserIDByToken(ctx context.Context, token string) (string, error)
	GetSession(ctx context.Context, sid string) (Session, error)
}
