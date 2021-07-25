package repository

import (
	"context"

	session2 "github.com/88labs/go-utils/auth/session"

	"github.com/88labs/go-utils/cerrors"

	"github.com/google/uuid"
)

// MemorySessionRepository セッション管理のin-memory版。
// テスト時にも便利なので公開しておく
type MemorySessionRepository struct {
	sessions map[string]*session2.Session
}

func (m *MemorySessionRepository) GetSession(ctx context.Context, id string) (*session2.Session, error) {
	if v, ok := m.sessions[id]; ok {
		return v, nil
	} else {
		return nil, cerrors.Newf(cerrors.NotFoundErr, nil, "unknown session id: %s", id)
	}
}

func (m *MemorySessionRepository) CreateSession(ctx context.Context, s *session2.Session) (string, error) {
	s2 := session2.FromRawSession(
		uuid.NewString(),
		s.UserId(),
		s.ClientId(),
		s.Token(),
		s.ExpiredAt(),
	)
	m.sessions[s2.Id()] = s2
	return s2.Id(), nil
}

func (m *MemorySessionRepository) DeleteSession(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func NewMemorySessionRepository() *MemorySessionRepository {
	return &MemorySessionRepository{
		sessions: map[string]*session2.Session{},
	}
}
