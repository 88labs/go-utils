package grpc

import (
	"context"
	"net/http"

	"github.com/88labs/go-utils/auth/protocol"
	session2 "github.com/88labs/go-utils/auth/session"

	"github.com/88labs/go-utils/cerrors"

	"google.golang.org/grpc/metadata"
)

func NewAuthFunc(sessionCookieName string, sessionRepository protocol.SessionRepository) func(ctx context.Context) (context.Context, error) {
	return func(ctx context.Context) (context.Context, error) {
		// Cookieを取得する
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, cerrors.New(cerrors.UnauthenticatedErr, nil, "metadataが取得できません")
		}

		rawCookies := md.Get("Cookie")
		if len(rawCookies) == 0 {
			return nil, cerrors.New(cerrors.UnauthenticatedErr, nil, "Cookieが取得できません")
		}

		header := http.Header{}
		header.Add("Cookie", rawCookies[0])
		request := http.Request{Header: header}

		sessionCookie, err := request.Cookie(sessionCookieName)
		if err != nil {
			return nil, cerrors.New(cerrors.UnauthenticatedErr, err, "セッションCookieが取得できませんでした")
		}

		sessionId := sessionCookie.Value
		s, err := sessionRepository.GetSession(ctx, sessionId)
		if err != nil {
			return nil, cerrors.New(cerrors.UnauthenticatedErr, err, "セッションCookieが不正です")
		}

		return NewContext(ctx, s), nil
	}
}

type ctxKeySession struct{}

// NewContext
// 公開しているのはテスト用。
func NewContext(ctx context.Context, session *session2.Session) context.Context {
	return context.WithValue(ctx, &ctxKeySession{}, session)
}

func FromContext(ctx context.Context) (*session2.Session, bool) {
	s, ok := ctx.Value(&ctxKeySession{}).(*session2.Session)
	return s, ok
}
