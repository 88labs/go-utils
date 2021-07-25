package grpc_test

import (
	"context"
	"fmt"
	"time"

	grpc2 "github.com/88labs/andpad-approval-bff/auth/grpc"
	"github.com/88labs/andpad-approval-bff/auth/protocol"
	repository2 "github.com/88labs/andpad-approval-bff/auth/repository"
	session2 "github.com/88labs/andpad-approval-bff/auth/session"

	"golang.org/x/oauth2"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
)

var _ = Describe("AuthFunc", func() {
	var authFunc func(ctx context.Context) (context.Context, error)
	var sessionRepository protocol.SessionRepository

	BeforeEach(func() {
		sessionRepository = repository2.NewMemorySessionRepository()
		authFunc = grpc2.NewAuthFunc("__Secure-SID", sessionRepository)
	})

	Context("セッションCookieあり", func() {
		var (
			ctx              context.Context
			requestSessionId string
			token            *oauth2.Token
		)

		BeforeEach(func() {
			ctx = context.Background()
			token = &oauth2.Token{
				AccessToken:  "AccessToken",
				RefreshToken: "RefreshToken",
				Expiry:       time.Now(),
			}

			s := session2.New(1, 2, token)
			requestSessionId, _ = sessionRepository.CreateSession(ctx, s)
			md := metadata.Pairs("cookie", fmt.Sprintf("%s=%s; web_access_token=piyo", "__Secure-SID", requestSessionId))
			ctx = metautils.NiceMD(md).ToIncoming(ctx)
		})

		It("認証成功 + セッションID取得できる", func() {
			ctx, err := authFunc(ctx)
			Expect(err).ShouldNot(HaveOccurred())
			session, ok := grpc2.FromContext(ctx)
			Expect(session.Id()).Should(Equal(requestSessionId))
			Expect(session.UserId()).Should(Equal(int32(1)))
			Expect(session.ClientId()).Should(Equal(int32(2)))
			Expect(session.Token()).Should(Equal(token))
			Expect(ok).Should(BeTrue())
		})
	})

	Context("セッションCookieあるが不正", func() {
		var (
			ctx              context.Context
			requestSessionId string
		)

		BeforeEach(func() {
			ctx = context.Background()
			requestSessionId = "invalid_session_id"
			md := metadata.Pairs("cookie", fmt.Sprintf("%s=%s; web_access_token=piyo", "__Secure-SID", requestSessionId))
			ctx = metautils.NiceMD(md).ToIncoming(ctx)
		})

		It("認証失敗", func() {
			_, err := authFunc(ctx)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("メタデータなし", func() {
		It("認証失敗", func() {
			_, err := authFunc(context.TODO())
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("Cookieなし", func() {
		It("認証失敗", func() {
			ctx := context.Background()
			md := metadata.Pairs("foo", "bar")
			ctx = metautils.NiceMD(md).ToIncoming(ctx)
			_, err := authFunc(ctx)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("CookieにセッションIDなし", func() {
		It("認証失敗", func() {
			ctx := context.Background()
			md := metadata.Pairs("cookie", "web_access_token=piyo")
			ctx = metautils.NiceMD(md).ToIncoming(ctx)
			_, err := authFunc(ctx)
			Expect(err).Should(HaveOccurred())
		})
	})
})
