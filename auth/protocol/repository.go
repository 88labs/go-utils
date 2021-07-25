//go:generate mockgen -source=$GOFILE -destination=mock/$GOFILE -package=mock_$GOPACKAGE

package protocol

import (
	"context"

	session2 "github.com/88labs/andpad-approval-bff/auth/session"
)

type SessionRepository interface {
	CreateSession(ctx context.Context, s *session2.Session) (string, error)
	GetSession(ctx context.Context, id string) (*session2.Session, error)
	DeleteSession(ctx context.Context, id string) error
}
