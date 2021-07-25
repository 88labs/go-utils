package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	mock_protocol "github.com/88labs/andpad-approval-bff/auth/protocol/mock"

	config2 "github.com/88labs/andpad-approval-bff/auth/config"
	"github.com/88labs/andpad-approval-bff/auth/protocol"
	"github.com/88labs/andpad-approval-bff/auth/session"

	"github.com/coreos/go-oidc"

	"golang.org/x/oauth2"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AuthHandler", func() {
	var (
		mux                   http.Handler
		writer                *httptest.ResponseRecorder
		authHandler           *AuthHandler
		conf                  config2.AuthConfig
		ctrl                  *gomock.Controller
		mockSessionRepository *mock_protocol.MockSessionRepository
		mockOAuth2Config      *mock_protocol.MockOAuth2Config
		mockVerifier          *mock_protocol.MockIDTokenVerifier
		mockUserInfoProvider  *mock_protocol.MockUserInfoProvider
		sm                    *gorillaStateManager
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockSessionRepository = mock_protocol.NewMockSessionRepository(ctrl)
		mockOAuth2Config = mock_protocol.NewMockOAuth2Config(ctrl)
		mockVerifier = mock_protocol.NewMockIDTokenVerifier(ctrl)
		mockUserInfoProvider = mock_protocol.NewMockUserInfoProvider(ctrl)

		conf = config2.AuthConfig{
			AppUrl: "https://mock/authenticated",
			CookieConfig: config2.CookieConfig{
				SessionCookieName: "mock_session",
			},
			AppUnauthenticatedUrl: "https://mock/unauthenticated",
		}
		sm = newCookieStateManager([]byte("authkey123"), []byte("enckey12341234567890123456789012"))

		authHandler = &AuthHandler{
			sessionRepository: mockSessionRepository,
			oAuth2Config:      mockOAuth2Config,
			config:            conf,
			stateManager:      sm,
			verifier:          mockVerifier,
			userInfoProvider:  mockUserInfoProvider,
		}

		mux = authHandler.RouteHttpServer()
		writer = httptest.NewRecorder()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("HealthCheck", func() {
		It("OKが返ること", func() {
			request, _ := http.NewRequest("GET", "/auth/health", nil)
			mux.ServeHTTP(writer, request)
			Expect(writer.Code).Should(Equal(http.StatusOK))
		})
	})

	Describe("Login", func() {
		Context("ログイン済みでない + ステート生成成功", func() {
			var (
				authCodeUrl = "https://oidc/auth"
				actualState string
			)

			BeforeEach(func() {
				mockOAuth2Config.EXPECT().
					AuthCodeURL(gomock.Any()).
					Do(func(state string) {
						actualState = state
					}).
					Return(authCodeUrl).
					Times(1)
			})

			It("OIDCのURLにリダイレクトされ、stateがCookieに設定されること", func() {
				request, _ := http.NewRequest("GET", "/auth/login", nil)
				mux.ServeHTTP(writer, request)

				// 所定のURLにリダイレクトされること
				Expect(writer.Code).Should(Equal(http.StatusFound))
				Expect(writer.Header().Get("Location")).Should(Equal(authCodeUrl))

				// stateをCookieに保持しようとすること
				setCookie := getCookie(writer.Header().Get("Set-Cookie"), stateCookieName)
				Expect(setCookie.Name).Should(Equal(stateCookieName))
				Expect(setCookie.Value).ShouldNot(BeEmpty())

				// URLのstateとCookieのstateが一致していること
				h := http.Header{}
				h.Add("Cookie", setCookie.String())
				r := &http.Request{Header: h}
				s, _ := sm.store.Get(r, stateCookieName)
				Expect(actualState).Should(Equal(s.Values[stateValueKey]))
			})
		})

		Context("ログイン済み", func() {
			var (
				request          *http.Request
				requestSessionId = "SessionId"
			)

			BeforeEach(func() {
				mockSessionRepository.EXPECT().
					GetSession(gomock.Any(), requestSessionId).
					Return(&session.Session{}, nil).
					Times(1)

				request, _ = http.NewRequest("GET", "/auth/login", nil)
				cookie := makeMockCookie(conf.SessionCookieName, requestSessionId)
				request.Header.Set("Cookie", cookie.String())
			})

			It("認証成功後のURLにリダイレクトされること", func() {
				mux.ServeHTTP(writer, request)
				// 認証成功後のURLにリダイレクトされること
				Expect(writer.Code).Should(Equal(http.StatusFound))
				Expect(writer.Header().Get("Location")).Should(Equal(conf.AppUrl))
			})
		})

		Context("ログイン済みでない + ステート生成失敗", func() {
			BeforeEach(func() {
				authHandler.stateManager = &gorillaStateManager{
					store: &errorStore{},
				}
			})

			It("Internal Server Error", func() {
				request, _ := http.NewRequest("GET", "/auth/login", nil)
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Callback", func() {
		var (
			request *http.Request
			code    = "code"
			token   = (&oauth2.Token{}).WithExtra(map[string]interface{}{
				"id_token": "id_token",
			})
			tokenSource oauth2.TokenSource
			userInfo    = protocol.UserInfo{
				Subject: "1",
				ID:      1,
				Client: struct {
					Id int32 `json:"id"`
				}{Id: 2},
			}
			sessionId = "session-id"
		)

		Context("正常系", func() {
			BeforeEach(func() {
				request = buildCallbackHttpRequest(authHandler, code, nil)

				tokenSource = (&oauth2.Config{}).TokenSource(request.Context(), token)

				mockOAuth2Config.EXPECT().
					Exchange(gomock.Any(), code).
					Return(token, nil).
					Times(1)

				mockVerifier.EXPECT().
					Verify(gomock.Any(), "id_token").
					Return(&oidc.IDToken{}, nil).
					Times(1)

				mockOAuth2Config.EXPECT().
					TokenSource(gomock.Any(), token).
					Return(tokenSource).
					Times(1)

				mockUserInfoProvider.EXPECT().
					UserInfo(gomock.Any(), tokenSource).
					Return(&userInfo, nil).
					Times(1)

				mockSessionRepository.EXPECT().
					CreateSession(gomock.Any(), session.New(int32(userInfo.ID), userInfo.Client.Id, token)).
					Return(sessionId, nil).
					Times(1)
			})

			It("認証成功後のURLにリダイレクト + セッションCookieの設定", func() {
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusFound))
				Expect(writer.Header().Get("Location")).Should(Equal(conf.AppUrl))

				// CookieのStateが消えるのは、stateManagerのUnitTestで担保
				// CookieのセッションIDが設定されるかのみ検証
				rawSessionCookie := writer.Header().Values("Set-Cookie")[1]
				Expect(rawSessionCookie).Should(ContainSubstring("Path=/"))
				Expect(rawSessionCookie).Should(ContainSubstring("HttpOnly"))
				Expect(rawSessionCookie).Should(ContainSubstring("Secure"))
				Expect(rawSessionCookie).Should(ContainSubstring("SameSite=Lax"))
				sessionCookie := getCookie(rawSessionCookie, conf.SessionCookieName)
				Expect(sessionCookie.Name).Should(Equal(conf.SessionCookieName))
				Expect(sessionCookie.Value).Should(Equal(sessionId))
			})
		})

		Context("セッション作成失敗", func() {
			BeforeEach(func() {
				request = buildCallbackHttpRequest(authHandler, code, nil)

				tokenSource = (&oauth2.Config{}).TokenSource(request.Context(), token)

				mockOAuth2Config.EXPECT().
					Exchange(gomock.Any(), code).
					Return(token, nil).
					Times(1)

				mockVerifier.EXPECT().
					Verify(gomock.Any(), "id_token").
					Return(&oidc.IDToken{}, nil).
					Times(1)

				mockOAuth2Config.EXPECT().
					TokenSource(gomock.Any(), token).
					Return(tokenSource).
					Times(1)

				mockUserInfoProvider.EXPECT().
					UserInfo(gomock.Any(), tokenSource).
					Return(&userInfo, nil).
					Times(1)

				mockSessionRepository.EXPECT().
					CreateSession(gomock.Any(), session.New(int32(userInfo.ID), userInfo.Client.Id, token)).
					Return(sessionId, errors.New("test")).
					Times(1)
			})

			It("Internal Server Errorが起きてセッションCookieが設定されないこと", func() {
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusInternalServerError))

				// CookieのStateが消えるのは、stateManagerのUnitTestで担保

				// CookieのセッションIDが設定されないこと
				cookies := writer.Header().Values("Set-Cookie")
				Expect(cookies).Should(HaveLen(1))
				sessionCookie := getCookie(cookies[0], conf.SessionCookieName)
				Expect(sessionCookie).Should(BeNil())
			})
		})

		Context("UserInfo取得失敗", func() {
			BeforeEach(func() {
				request = buildCallbackHttpRequest(authHandler, code, nil)

				tokenSource = (&oauth2.Config{}).TokenSource(request.Context(), token)

				mockOAuth2Config.EXPECT().
					Exchange(gomock.Any(), code).
					Return(token, nil).
					Times(1)

				mockVerifier.EXPECT().
					Verify(gomock.Any(), "id_token").
					Return(&oidc.IDToken{}, nil).
					Times(1)

				mockOAuth2Config.EXPECT().
					TokenSource(gomock.Any(), token).
					Return(tokenSource).
					Times(1)

				mockUserInfoProvider.EXPECT().
					UserInfo(gomock.Any(), tokenSource).
					Return(&userInfo, errors.New("test")).
					Times(1)
			})

			It("Internal Server Errorが起きてセッションCookieが設定されないこと", func() {
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusInternalServerError))

				// CookieのStateが消えるのは、stateManagerのUnitTestで担保

				// CookieのセッションIDが設定されないこと
				cookies := writer.Header().Values("Set-Cookie")
				Expect(cookies).Should(HaveLen(1))
				sessionCookie := getCookie(cookies[0], conf.SessionCookieName)
				Expect(sessionCookie).Should(BeNil())
			})
		})

		Context("Verify失敗", func() {
			BeforeEach(func() {
				request = buildCallbackHttpRequest(authHandler, code, nil)

				tokenSource = (&oauth2.Config{}).TokenSource(request.Context(), token)

				mockOAuth2Config.EXPECT().
					Exchange(gomock.Any(), code).
					Return(token, nil).
					Times(1)

				mockVerifier.EXPECT().
					Verify(gomock.Any(), "id_token").
					Return(&oidc.IDToken{}, errors.New("test")).
					Times(1)
			})

			It("Unauthorizedエラー + セッションCookieが設定されないこと", func() {
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusUnauthorized))

				// CookieのStateが消えるのは、stateManagerのUnitTestで担保

				// CookieのセッションIDが設定されないこと
				cookies := writer.Header().Values("Set-Cookie")
				Expect(cookies).Should(HaveLen(1))
				sessionCookie := getCookie(cookies[0], conf.SessionCookieName)
				Expect(sessionCookie).Should(BeNil())
			})
		})

		Context("ID Tokenがない", func() {
			BeforeEach(func() {
				request = buildCallbackHttpRequest(authHandler, code, nil)

				mockOAuth2Config.EXPECT().
					Exchange(gomock.Any(), code).
					Return(&oauth2.Token{}, nil).
					Times(1)
			})

			It("Unauthorizedエラー + セッションCookieが設定されないこと", func() {
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusUnauthorized))

				// CookieのStateが消えるのは、stateManagerのUnitTestで担保

				// CookieのセッションIDが設定されないこと
				cookies := writer.Header().Values("Set-Cookie")
				Expect(cookies).Should(HaveLen(1))
				sessionCookie := getCookie(cookies[0], conf.SessionCookieName)
				Expect(sessionCookie).Should(BeNil())
			})
		})

		Context("認可コードからAccessTokenを取得できず", func() {
			BeforeEach(func() {
				request = buildCallbackHttpRequest(authHandler, code, nil)

				mockOAuth2Config.EXPECT().
					Exchange(gomock.Any(), code).
					Return(nil, errors.New("test")).
					Times(1)
			})

			It("Unauthorizedエラー + セッションCookieが設定されないこと", func() {
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusUnauthorized))

				// CookieのStateが消えるのは、stateManagerのUnitTestで担保

				// CookieのセッションIDが設定されないこと
				cookies := writer.Header().Values("Set-Cookie")
				Expect(cookies).Should(HaveLen(1))
				sessionCookie := getCookie(cookies[0], conf.SessionCookieName)
				Expect(sessionCookie).Should(BeNil())
			})
		})

		Context("state不正", func() {
			BeforeEach(func() {
				invalidState := ""
				request = buildCallbackHttpRequest(authHandler, code, &invalidState)
			})

			It("Unauthorizedエラー + セッションCookieが設定されないこと", func() {
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusUnauthorized))

				// CookieのStateが消えるのは、stateManagerのUnitTestで担保

				// CookieのセッションIDが設定されないこと
				cookies := writer.Header().Values("Set-Cookie")
				Expect(cookies).Should(HaveLen(1))
				sessionCookie := getCookie(cookies[0], conf.SessionCookieName)
				Expect(sessionCookie).Should(BeNil())
			})
		})
	})

	Describe("Logout", func() {
		Context("セッションCookieあり", func() {
			requestSessionId := "SessionId"
			var (
				deleteSessionId string
				request         *http.Request
			)

			BeforeEach(func() {
				mockSessionRepository.EXPECT().
					DeleteSession(gomock.Any(), gomock.Any()).
					Do(func(ctx context.Context, id string) {
						deleteSessionId = id
					}).
					Return(errors.New("dummy")).
					Times(1)

				request, _ = http.NewRequest("GET", "/auth/logout", nil)
				cookie := makeMockCookie(conf.SessionCookieName, requestSessionId)
				request.Header.Set("Cookie", cookie.String())
			})

			It("クッキーが消されて所定のURLにリダイレクトされること", func() {
				mux.ServeHTTP(writer, request)

				// 所定のURLにリダイレクトされること
				Expect(writer.Code).Should(Equal(http.StatusFound))
				Expect(writer.Header().Get("Location")).Should(Equal(conf.AppUnauthenticatedUrl))

				// セッションクッキーが消えること
				rawSetCookie := writer.Header().Get("Set-Cookie")
				Expect(rawSetCookie).Should(ContainSubstring("Max-Age=0"))
				setCookie := getCookie(rawSetCookie, conf.SessionCookieName)
				Expect(setCookie.Name).Should(Equal(conf.SessionCookieName))
				Expect(setCookie.Value).Should(Equal(""))

				// セッション本体の削除が試行されること
				Expect(deleteSessionId).Should(Equal(requestSessionId))
			})
		})

		Context("セッションCookieなし", func() {
			var (
				request *http.Request
			)

			BeforeEach(func() {
				request, _ = http.NewRequest("GET", "/auth/logout", nil)
			})

			It("ログアウト後のURLにリダイレクトされること", func() {
				mux.ServeHTTP(writer, request)
				Expect(writer.Code).Should(Equal(http.StatusFound))
				Expect(writer.Header().Get("Location")).Should(Equal(conf.AppUnauthenticatedUrl))
			})
		})
	})

})

func makeMockCookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	}
}

func getCookie(rawCookie, name string) *http.Cookie {
	header := http.Header{}
	header.Add("Cookie", rawCookie)
	request := http.Request{Header: header}
	c, _ := request.Cookie(name)
	return c
}

func buildCallbackHttpRequest(authHandler *AuthHandler, code string, urlState *string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()
	state, _ := authHandler.stateManager.Issue(w, r)
	cookie := getCookie(w.Header().Get("Set-Cookie"), stateCookieName)

	actualUrlState := state
	if urlState != nil {
		actualUrlState = *urlState
	}

	request, _ := http.NewRequest("GET", fmt.Sprintf("/auth/callback?code=%v&state=%v", code, actualUrlState), nil)
	request.Header = http.Header{}
	request.Header.Add("Cookie", cookie.String())
	return request
}
