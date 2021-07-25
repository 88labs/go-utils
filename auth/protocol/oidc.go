//go:generate mockgen -source=$GOFILE -destination=mock/$GOFILE -package=mock_$GOPACKAGE

package protocol

import (
	"context"
	"net/http"

	"github.com/coreos/go-oidc"

	"golang.org/x/oauth2"
)

type OAuth2Config interface {
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource
}

type IDTokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

type StateManager interface {
	Issue(w http.ResponseWriter, r *http.Request) (string, error)
	Verify(w http.ResponseWriter, r *http.Request) error
}

type UserInfoProvider interface {
	UserInfo(ctx context.Context, tokenSource oauth2.TokenSource) (*UserInfo, error)
}

// UserInfo ANDPADのOIDCのUserInfo
//
// {
//    "sub": "123458",
//    "iss": "https://andpad.jp",
//    "name": "菊地遥菜",
//    "email": "ayumu.kanechika@88oct.co.jp",
//    "phone_number": "063-533-3258",
//    "picture": "https://staging-cloudfront.andpaddev.xyz/assets/common/images/defaults/user/profile_image_filename/office/large_pht-dummy-office_03-4095c7a2e84bee73bab91364df75f5a89d3611fd2e6479706ec995c1bf151b84.png",
//    "client": {
//        "id": 123456,
//        "name": "株式会社あんどぱっど"
//    }
//}
type UserInfo struct {
	Subject string `json:"sub"`

	// Subjectをintに変換したもの
	ID int

	// ユーザの所属会社の情報(ANDPAD独自)
	Client struct {
		Id int32 `json:"id"`
	} `json:"client"`
}
