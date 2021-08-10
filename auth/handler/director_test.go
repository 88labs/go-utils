package handler

import (
	"net/http"
	"net/http/httptest"
	url2 "net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cookieDirector", func() {
	var (
		director   *cookieDirector
		writer     *httptest.ResponseRecorder
		defaultUrl = "https://approval.andpad.jp/"
	)

	BeforeEach(func() {
		director = newCookieDirector(defaultUrl, "redirect_url", []byte("authkey123"), []byte("enckey12341234567890123456789012"))
		writer = httptest.NewRecorder()
	})

	Describe("GetUrlFromParams", func() {
		It("redirectUrlが指定されていたら、それが取得できること", func() {
			redirectUrl := "https://approval.andpad.jp/applications/1"
			request, _ := http.NewRequest("GET", "https://approval-api.andapd.jp/auth/login?redirect_url="+url2.QueryEscape(redirectUrl), nil)
			url := director.GetUrlFromParams(request)
			Expect(url).Should(Equal(redirectUrl))
		})

		It("redirectUrlが指定されていなければ、デフォルト値が取得できること", func() {
			request, _ := http.NewRequest("GET", "https://approval-api.andapd.jp/auth/login", nil)
			url := director.GetUrlFromParams(request)
			Expect(url).Should(Equal(director.defaultUrl))
		})
	})

	Describe("Set", func() {
		var (
			redirectUrl string
			request     *http.Request
		)
		BeforeEach(func() {
			redirectUrl = "https://approval.andpad.jp/applications/1"
			request, _ = http.NewRequest("GET", "https://approval-api.andapd.jp/auth/login?redirect_url="+url2.QueryEscape(redirectUrl), nil)
		})

		It("redirectUrlをCookieに記憶できること", func() {
			err := director.SetUrl(writer, request, redirectUrl)
			Expect(err).ShouldNot(HaveOccurred())

			s, err := director.store.Get(request, redirectCookieName)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(s.Values[redirectValueKey]).Should(Equal(redirectUrl))
			Expect(s.Options.HttpOnly).Should(BeTrue())
			Expect(s.Options.Secure).Should(BeTrue())
			Expect(s.Options.SameSite).Should(Equal(http.SameSiteStrictMode))
			Expect(s.Options.MaxAge).Should(Equal(0))
		})

		It("RedirectUrlの設定失敗", func() {
			director.store = &errorStore{}
			err := director.SetUrl(writer, request, redirectUrl)
			Expect(err).Should(HaveOccurred())
		})

		It("RedirectUrlの設定失敗(store取得失敗)", func() {
			director = newCookieDirector("https://approval.andpad.jp/", "redirect_url", []byte("authkey123"), []byte("wrong key"))
			err := director.SetUrl(writer, request, redirectUrl)
			Expect(err).Should(HaveOccurred())
		})
	})

	Describe("GetUrl", func() {
		It("redirectUrlがCookieから取得できたら、それが取得できること", func() {
			redirectUrl := "https://approval.andpad.jp/applications/1"
			// Arrange
			request, _ := http.NewRequest("GET", "/auth/callback?state=state", nil)
			s, _ := director.store.Get(request, redirectCookieName)
			s.Values[redirectValueKey] = redirectUrl
			_ = s.Save(request, writer)

			// Act
			url := director.GetUrl(writer, request)

			// Assert
			Expect(url).Should(Equal(redirectUrl))
			s, _ = director.store.Get(request, redirectCookieName)
			Expect(s.Options.MaxAge).Should(Equal(-1))
		})

		It("redirectUrlがCookieから取得できないなら、デフォルトURLを返す", func() {
			// Arrange
			request, _ := http.NewRequest("GET", "/auth/callback?state=state", nil)

			// Act
			url := director.GetUrl(writer, request)

			// Assert
			Expect(url).Should(Equal(defaultUrl))
		})

		It("store取得失敗したら、デフォルトURLを返す", func() {
			// Arrange
			director.store = &errorStore{}
			request, _ := http.NewRequest("GET", "/auth/callback?state=state", nil)

			// Act
			url := director.GetUrl(writer, request)

			// Assert
			Expect(url).Should(Equal(defaultUrl))
		})
	})
})
