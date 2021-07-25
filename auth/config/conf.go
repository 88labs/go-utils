package config

import "github.com/kelseyhightower/envconfig"

// AuthConfig モジュールの動作設定
//
// 大別して3つの設定に分かれる。
// - ANDPADのOIDC由来の設定
// - モジュールが使うCookieの設定
// - 上記以外のモジュールの動作設定
//
// FIXME envconfigの記述やめる。変数名から自動判定できるので。
type AuthConfig struct {
	OidcConfig
	CookieConfig

	// OIDCの認証・認可後に開くアプリのURL。サーバURL -> アプリのURLの順番で開く。
	AppUrl string `envconfig:"APP_URL" default:"https://approval.andpaddev.xyz/"`
	// ログアウト後に開くアプリのURL
	AppUnauthenticatedUrl string `envconfig:"APP_UNAUTHENTICATED_URL" default:"https://approval.andpaddev.xyz/"`
}

type CookieConfig struct {
	// セッションクッキーの名前
	SessionCookieName string `envconfig:"SESSION_COOKIE_NAME" default:"__Secure-SID"`

	// クッキーの署名用のキー
	//
	// OAuth2のstateを発行するときに使っている。
	// 32文字のランダム文字列を設定する(プロダクトごとの機密情報)。
	CookieSignKey string `envconfig:"COOKIE_SIGN_KEY"`

	// クッキーの暗号化用のキー
	//
	// OAuth2のstateを発行するときに使っている。
	// 32文字のランダム文字列を設定する(プロダクトごとの機密情報)。
	CookieEncryptionKey string `envconfig:"COOKIE_ENCRYPTION_KEY"`
}

type OidcConfig struct {
	// Open ID ConnectのRPとしてのクライアントID
	ClientID string `envconfig:"CLIENT_ID" default:"D6b9ZdnpBwd8l0569gDCq72sQ0G0ESO8wtgy7flY4Sc"`
	// OIDCのRPとしてのクライアントシークレット(プロダクトごとの機密情報)
	ClientSecret string `envconfig:"CLIENT_SECRET" default:"fGp2cEE7MMWZh-9WHiCb5SvNhswFdPpNKFzEGUO_s4I"`
	// OIDCのIssuer。IDTokenの検証に使う。
	// 現状は、環境によらない固定値なのでdefault値のままでOK。
	IssuerUrl string `envconfig:"ISSUER_URL" default:"https://andpad.jp"`
	// OIDCの認証/認可画面のURL。
	AuthUrl string `envconfig:"AUTH_URL" default:"https://staging2.api.andpaddev.xyz/v3/auth/oauth/authorize"`
	// OIDCのトークン発行APIのURL。
	TokenUrl string `envconfig:"TOKEN_URL" default:"https://staging2.api.andpaddev.xyz/v3/auth/oauth/token"`
	// OIDCの認可サーバの公開鍵のURL。
	JwksUrl string `envconfig:"JWKS_URL" default:"https://staging2.api.andpaddev.xyz/v3/auth/oauth/discovery/keys"`
	// OIDCのUserInfoのURL。
	UserInfoUrl string `envconfig:"USER_INFO_URL" default:"https://staging2.api.andpaddev.xyz/v3/auth/oauth/userinfo"`
	// OIDCの認証・認可後に開くサーバのURL。
	// デフォルト値のPort番号がWebPortのものと異なるのは、こちらのPort番号はhttps用Proxyのものだから。
	CallbackUrl string `envconfig:"CALLBACK_URL" default:"https://local.approval-api.andpaddev.xyz:8003/auth/callback"`
}

func LoadAuthConfig(prefix string) *AuthConfig {
	c := &AuthConfig{}

	if err := envconfig.Process(prefix, c); err != nil {
		panic(err)
	}

	return c
}
