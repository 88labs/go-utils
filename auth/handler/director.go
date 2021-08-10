package handler

import (
	"net/http"

	"github.com/88labs/go-utils/cerrors"

	"github.com/gorilla/sessions"
)

const redirectCookieName = "__Secure-REDIRECT_URL"
const redirectValueKey = "url"

type cookieDirector struct {
	store      sessions.Store
	defaultUrl string
	key        string
}

func newCookieDirector(defaultUrl string, key string, keyPairs ...[]byte) *cookieDirector {
	store := sessions.NewCookieStore(keyPairs...)
	store.Options.Secure = true
	store.Options.HttpOnly = true
	store.Options.SameSite = http.SameSiteStrictMode
	store.Options.MaxAge = 0

	return &cookieDirector{
		store:      store,
		defaultUrl: defaultUrl,
		key:        key,
	}
}

func (d cookieDirector) GetUrlFromParams(r *http.Request) string {
	if v := r.FormValue(d.key); len(v) != 0 {
		return v
	} else {
		return d.defaultUrl
	}
}

func (d cookieDirector) SetUrl(w http.ResponseWriter, r *http.Request, redirectUrl string) error {
	session, err := d.store.Get(r, redirectCookieName)
	if err != nil {
		return cerrors.New(cerrors.UnknownErr, err, "認証後のURL保存用のセッションの取得失敗")
	}
	session.Values[redirectValueKey] = redirectUrl

	err = session.Save(r, w)
	if err != nil {
		return cerrors.New(cerrors.UnknownErr, err, "認証後のURLの保存失敗")
	} else {
		return nil
	}
}

func (d cookieDirector) GetUrl(w http.ResponseWriter, r *http.Request) string {
	session, err := d.store.Get(r, redirectCookieName)
	if err != nil {
		return d.defaultUrl
	}

	defer func() {
		session.Options.MaxAge = -1
		_ = session.Save(r, w)
	}()

	if v, ok := session.Values[redirectValueKey].(string); ok {
		return v
	} else {
		return d.defaultUrl
	}
}
