// +build medium_test

package repository

import (
	"context"
	"time"

	"github.com/88labs/andpad-approval-bff/auth/session"
	"github.com/bxcodec/faker/v3"
	"golang.org/x/oauth2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DynamoDBSessionRepository", func() {
	var (
		ctx      context.Context
		repo     *DynamoDBSessionRepository
		userId   int32
		clientId int32
		token    *oauth2.Token
	)
	BeforeEach(func() {
		ctx = context.Background()
		repo = NewDynamoDBSessionRepository(DynamoDBConfig{
			DBType:       DBTypeDynamoDBLocal,
			Region:       "ap-northeast-1",
			Endpoint:     "http://localhost:8005",
			SessionTable: "approval_session_local",
		})
		userId = 1
		clientId = 2
		token = &oauth2.Token{}
		_ = faker.FakeData(token)
	})

	It("セッションの保存/取得/削除", func() {
		now := time.Now().Unix()

		id, err := repo.CreateSession(ctx, session.New(userId, clientId, token))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(id).ShouldNot(BeEmpty())

		s, err := repo.GetSession(ctx, id)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(s.Id()).Should(Equal(id))
		Expect(s.UserId()).Should(Equal(userId))
		Expect(s.ClientId()).Should(Equal(clientId))
		Expect(s.Token().AccessToken).Should(Equal(token.AccessToken))
		Expect(s.Token().RefreshToken).Should(Equal(token.RefreshToken))
		Expect(s.Token().Expiry.Unix()).Should(Equal(token.Expiry.Unix()))
		Expect(s.ExpiredAt() > now).Should(BeTrue())

		err = repo.DeleteSession(ctx, id)
		Expect(err).ShouldNot(HaveOccurred())

		s, err = repo.GetSession(ctx, id)
		Expect(err).Should(HaveOccurred())
		Expect(s).Should(BeNil())
	})
})
