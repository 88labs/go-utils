package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/sessions"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cookieStateManager", func() {
	var (
		sm     *gorillaStateManager
		writer *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		sm = newCookieStateManager([]byte("authkey123"), []byte("enckey12341234567890123456789012"))
		writer = httptest.NewRecorder()
	})

	Describe("Issue", func() {
		It("stateは保存されること", func() {
			request, _ := http.NewRequest("GET", "/auth/login", nil)
			state, _ := sm.Issue(writer, request)

			s, err := sm.store.Get(request, stateCookieName)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(s.Values[stateValueKey]).Should(Equal(state))
			Expect(s.Options.HttpOnly).Should(BeTrue())
			Expect(s.Options.Secure).Should(BeTrue())
			Expect(s.Options.SameSite).Should(Equal(http.SameSiteStrictMode))
			Expect(s.Options.MaxAge).Should(Equal(3 * 60))
		})

		It("stateはランダム文字列", func() {
			request, _ := http.NewRequest("GET", "/auth/login", nil)

			state, err := sm.Issue(writer, request)
			state2, err2 := sm.Issue(writer, request)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(state).ShouldNot(BeEmpty())
			Expect(err2).ShouldNot(HaveOccurred())
			Expect(state2).ShouldNot(BeEmpty())
			Expect(state).ShouldNot(Equal(state2))
		})

		Context("state保存失敗", func() {
			It("エラーになること", func() {
				sm = newCookieStateManager([]byte("authkey123"), []byte("wrong key"))
				request, _ := http.NewRequest("GET", "/auth/login", nil)
				state, err := sm.Issue(writer, request)
				Expect(err).Should(HaveOccurred())
				Expect(state).Should(BeEmpty())
			})
		})

		Context("state取得失敗", func() {
			It("エラーになること", func() {

				sm = &gorillaStateManager{
					store: &errorStore{},
				}

				request, _ := http.NewRequest("GET", "/auth/login", nil)
				state, err := sm.Issue(writer, request)

				Expect(err).Should(HaveOccurred())
				Expect(state).Should(BeEmpty())
			})
		})
	})

	Describe("Verify", func() {
		It("stateが正しいなら成功", func() {
			// Arrange
			request, _ := http.NewRequest("GET", "/auth/callback?state=state", nil)
			s, _ := sm.store.Get(request, stateCookieName)
			s.Values[stateValueKey] = "state"
			_ = s.Save(request, writer)

			// Act
			err := sm.Verify(writer, request)

			// Assert
			Expect(err).ShouldNot(HaveOccurred())
			s, _ = sm.store.Get(request, stateCookieName)
			Expect(s.Options.MaxAge).Should(Equal(-1))
		})

		It("state不一致なら失敗", func() {
			// Arrange
			request, _ := http.NewRequest("GET", "/auth/callback?state=state", nil)
			s, _ := sm.store.Get(request, stateCookieName)
			s.Values[stateValueKey] = "foobar"
			_ = s.Save(request, writer)

			// Act
			err := sm.Verify(writer, request)

			// Assert
			Expect(err).Should(HaveOccurred())
			s, _ = sm.store.Get(request, stateCookieName)
			Expect(s.Options.MaxAge).Should(Equal(-1))
		})

		Context("state取得失敗", func() {
			It("エラーになること", func() {
				sm = &gorillaStateManager{
					store: &errorStore{},
				}
				request, _ := http.NewRequest("GET", "/auth/callback", nil)
				err := sm.Verify(writer, request)
				Expect(err).Should(HaveOccurred())
			})
		})
	})
})

type errorStore struct {
	sessions.CookieStore
}

func (store *errorStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return nil, errors.New("dummy")
}
