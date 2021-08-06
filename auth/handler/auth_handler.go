package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/88labs/go-utils/auth/config"
	"github.com/88labs/go-utils/auth/protocol"
	"github.com/88labs/go-utils/auth/session"

	"github.com/coreos/go-oidc"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
)

type AuthHandler struct {
	sessionRepository protocol.SessionRepository
	config            config.AuthConfig
	oAuth2Config      protocol.OAuth2Config
	stateManager      protocol.StateManager
	verifier          protocol.IDTokenVerifier
	userInfoProvider  protocol.UserInfoProvider
}

func NewAuthHandler(config config.AuthConfig,
	sessionRepository protocol.SessionRepository) *AuthHandler {
	// 外からctxもらった方が良い気もする
	ctx := context.Background()
	keySet := oidc.NewRemoteKeySet(ctx, config.JwksUrl)
	verifier := oidc.NewVerifier(
		config.IssuerUrl,
		keySet,
		&oidc.Config{
			ClientID: config.ClientID,
		},
	)

	sm := newCookieStateManager([]byte(config.CookieSignKey), []byte(config.CookieEncryptionKey))

	authConfig := oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.CallbackUrl,
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.AuthUrl,
			TokenURL: config.TokenUrl,
		},
		Scopes: []string{oidc.ScopeOpenID},
	}

	uip := NewUserInfoProvider(config.UserInfoUrl)

	return &AuthHandler{
		sessionRepository: sessionRepository,
		config:            config,
		oAuth2Config:      &authConfig,
		stateManager:      sm,
		verifier:          verifier,
		userInfoProvider:  uip,
	}
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintln(w, "OK")
}

func Logout(h *AuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessionCookie, err := r.Cookie(h.config.SessionCookieName); err == nil {
			_ = h.sessionRepository.DeleteSession(r.Context(), sessionCookie.Value)
			deleteCookie(w, h.config.SessionCookieName)
		}
		http.Redirect(w, r, h.config.AppUnauthenticatedUrl, http.StatusFound)
	}
}

func Login(h *AuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.isAlreadyLoggedIn(r) {
			http.Redirect(w, r, h.config.AppUrl, http.StatusFound)
			return
		}

		state, err := h.stateManager.Issue(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		authUrl := h.oAuth2Config.AuthCodeURL(state)
		http.Redirect(w, r, authUrl, http.StatusFound)
	}
}

func Callback(h *AuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h.stateManager.Verify(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		oauth2Token, err := h.oAuth2Config.Exchange(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "ユーザを認証できません", http.StatusUnauthorized)
			return
		}

		_, err = h.verifier.Verify(r.Context(), rawIDToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		tokenSource := h.oAuth2Config.TokenSource(r.Context(), oauth2Token)
		userInfo, err := h.userInfoProvider.UserInfo(r.Context(), tokenSource)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s := session.New(int32(userInfo.ID), userInfo.Client.Id, oauth2Token)
		sessionId, err := h.sessionRepository.CreateSession(r.Context(), s)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     h.config.SessionCookieName,
			Value:    sessionId,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(1 * 24 * time.Hour),
		})

		http.Redirect(w, r, h.config.AppUrl, http.StatusFound)
	}
}

func (h *AuthHandler) RouteHttpServer() (mux http.Handler) {
	r := chi.NewRouter()
	mux = r

	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", Login(h))
		r.Get("/callback", Callback(h))
		r.HandleFunc("/logout", Logout(h))
		r.Get("/health", HealthCheck)
	})

	return
}

func deleteCookie(w http.ResponseWriter, key string) {
	http.SetCookie(w, &http.Cookie{
		Name:     key,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (h *AuthHandler) isAlreadyLoggedIn(r *http.Request) bool {
	sessionCookie, err := r.Cookie(h.config.SessionCookieName)
	if err != nil {
		return false
	}

	sessionId := sessionCookie.Value
	_, err = h.sessionRepository.GetSession(r.Context(), sessionId)
	return err == nil
}
