# auth

ANDPAD OIDCを使った認証とセッション管理を実現するモジュールです。

## Feature

- OIDCによる認証
  - 認可方式は認可コードフロー、OIDCのRPはバックエンドが担う方式です
- 認証後のセッション管理
- GRPC API実行時の認証

## Quick Start 

In-Memory、stagingの場合で説明します。

### 1.認可サーバにRPの情報を登録する

https://andpad-dev.esa.io/posts/356

Callback URLには、https://example.com/auth/callback というような値を設定すること(詳細は後述)。

### 2.環境変数にモジュールの設定をする

OIDCのClient Secretなどは環境変数として設定する。
設定項目は、auth/config/conf.go を参照願います。
環境変数名のprefixには"ANDPAD_OIDC"をつける。

(prefixを別のものにしたり、そもそも環境変数を使わないようにすることも可能です)

### 3.AuthHandlerを作る

```go
sessionRepository := session.NewMemorySessionRepository()
c := config.LoadAuthConfig("ANDPAD_OIDC")
authHandler := auth_handler.NewAuthHandler(*c, sessionRepository)
```

### 4.Httpサーバを起動する

```go
mux := authHandler.RouteHttpServer()
ch <- http.Serve(listener, mux)
```

httpサーバには、4つのエントリポイントができます。

- /auth/login
  - 認可フローを開始する
- /auth/callback
  - 認可サーバから発行された認可コードを受け取り、ユーザ認証をする
  - **ここのURLを認可サーバのコールバックURLに設定してください**
- /auth/logout 
  - 認証セッションを消す
- /auth/health
  - ヘルスチェック用

### 5.GRPCの認証interceptorを設定する

```go
authFunc := grpc_auth.NewAuthFunc(authConfig.SessionCookieName, sessionRepository)

grpcServer := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        grpc_auth.UnaryServerInterceptor(authFunc),
    ),
    ...
)
```

### 6.Web Frontから認証を開始する

まず、/auth/loginを開いてください。
認証成功後は、grpc-web APIを呼べるようになります。