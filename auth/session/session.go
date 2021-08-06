package session

import (
	"time"

	"golang.org/x/oauth2"
)

type Session struct {
	id     string
	userId int32
	// ユーザの所属会社のID
	clientId int32
	// OIDCのトークン
	token *oauth2.Token
	// セッションの有効期限
	expiredAt int64
}

func New(userId int32,
	clientId int32,
	token *oauth2.Token) *Session {
	return &Session{
		userId:    userId,
		clientId:  clientId,
		token:     token,
		expiredAt: time.Now().Add(1 * 24 * time.Hour).Unix(),
	}
}

func FromRawSession(
	id string,
	userId int32,
	clientId int32,
	token *oauth2.Token,
	expiredAt int64,
) *Session {
	return &Session{
		id:        id,
		userId:    userId,
		clientId:  clientId,
		token:     token,
		expiredAt: expiredAt,
	}
}

func (s *Session) Id() string {
	return s.id
}

func (s *Session) UserId() int32 {
	return s.userId
}

func (s *Session) ClientId() int32 {
	return s.clientId
}

func (s *Session) Token() *oauth2.Token {
	return s.token
}

func (s *Session) ExpiredAt() int64 {
	return s.expiredAt
}
