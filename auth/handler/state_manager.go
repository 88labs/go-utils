package handler

import (
	"net/http"

	"github.com/88labs/go-utils/cerrors"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
)

const stateCookieName = "__Secure-STATE"
const stateValueKey = "state"
const stateMaxAge = 3 * 60

type gorillaStateManager struct {
	store sessions.Store
}

func newCookieStateManager(keyPairs ...[]byte) *gorillaStateManager {
	store := sessions.NewCookieStore(keyPairs...)
	store.Options.Secure = true
	store.Options.HttpOnly = true
	store.Options.SameSite = http.SameSiteStrictMode
	store.MaxAge(stateMaxAge)

	return &gorillaStateManager{
		store: store,
	}
}

func (s *gorillaStateManager) Issue(w http.ResponseWriter, r *http.Request) (string, error) {
	session, err := s.store.Get(r, stateCookieName)
	if err != nil {
		return "", cerrors.New(cerrors.UnknownErr, err, "state保存用のセッションの取得失敗")
	}

	state := uuid.NewString()
	session.Values[stateValueKey] = state

	err = session.Save(r, w)
	if err != nil {
		return "", cerrors.New(cerrors.UnknownErr, err, "stateの保存失敗")
	}

	return state, nil
}

func (s *gorillaStateManager) Verify(w http.ResponseWriter, r *http.Request) error {
	session, err := s.store.Get(r, stateCookieName)
	if err != nil {
		return cerrors.New(cerrors.UnauthenticatedErr, err, "state取得失敗")
	}

	defer func() {
		session.Options.MaxAge = -1
		_ = session.Save(r, w)
	}()

	urlState := r.URL.Query().Get(stateValueKey)
	if session.Values[stateValueKey] != urlState {
		return cerrors.New(cerrors.UnauthenticatedErr, err, "state不正")
	}

	return nil
}
