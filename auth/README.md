# auth

## Feature

- OIDCによる認証
  - 認可方式は認可コードフロー、OIDCのRPはバックエンドが担う方式です
- 認証後のセッション管理
- GRPC API実行時の認証

## Quick Start 

In-Memory、stagingの場合で説明します。
あくまで試用向けです。
サービスのプロセスが複数ある場合にはセッション情報が共有されませんし、プロセスが終了するとセッション情報は揮発します。

### 1.OIDCのAuthサーバに、認可後のコールバックの情報を登録する

Callback URLには、https://example.com/auth/callback というような値を設定すること(詳細は後述)。

### 2.環境変数にモジュールの設定をする

OIDCのClient Secretなどは環境変数として設定する。
設定項目は、auth/config/conf.go を参照願います。
環境変数名のprefixには"ANDPAD_OIDC"をつける。

(prefixを変えたり、環境変数を使わないようにすることも可能です)

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

RouteHttpServer()はよく使いそうな設定を関数化しただけです。
エントリポイントを変えたり、Webフレームワークと組み合わせるのは簡単にできるはず。

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

このinterceptorは、Contextにセッション情報を格納します。
格納したセッション情報を取り出すコードを示します。

```go
// このinterceptorを使っていると、okがfalseになることはないです
s, ok := auth.FromContext(ctx)
```

### 6.Web Frontから認証を開始する

まず、/auth/loginを開いてください。
認証成功後は、grpc-web APIを呼べるようになります。
認証失敗時は、エラーコードがUnauthenticatedになります。

## 本格的に導入する

Quick Startでは、セッション管理をIn-Memoryで行っていました。
これでは、複数のコンテナでセッションが共有されないですし、1コンテナだとしても再起動するとセッションは消えます。

現時点では、Production向けのセッション管理には、Dynamo DBを使うことができます。
その場合の利用手順を示します。

### 1.セッション管理用のDynamo DBのテーブルを作る

テーブル作成のパラメータは、ほぼ任意です。
ただし、TTLに使う項目は"ExpiredAt"にしてください。

### 2.Dynamo DBへのアクセス権限をもらう

モジュールを組み込むサービスに、Dynamo DBへのRead/Write権限を付与してもらってください。
EKSで動いているサービスなら、S3などにアクセスするときと類似の申請をすれば良いです。

### 3.Dynamo DBを使うためのパラメータを設定する

環境変数による設定例を示します。ここでは、環境変数名の接頭辞には"ANDPAD_OIDC"を付けます。

```
ANDPAD_OIDC_DYNAMO_DB_TYPE: "dynamodb"
ANDPAD_OIDC_DYNAMO_SESSION_TABLE: "your_table_name"
```

Dynamo DBを使うための設定項目の詳細は、[DynamoDBConfig](https://github.com/88labs/go-utils/blob/956f67bcd3e6b6c9eeab4d55d92566b9c5222a6b/auth/repository/dynamodb_session_repository.go#L39) を参照願います。

### 4.Dynamo DBを使う

```go
dbConfig := session.LoadSessionDBConfig("ANDPAD_OIDC")
sessionRepository := session.NewDynamoDBSessionRepository(*dbConfig)

authConfig := loadAuthConfig()
authHandler := auth_handler.NewAuthHandler(*authConfig, sessionRepository)

return &AuthRegistry{
    Auth:     authHandler,
    AuthFunc: grpc_auth.NewAuthFunc(authConfig.SessionCookieName, sessionRepository),
}
```

1. LoadSessionDBConfig()で設定を読み込みます
2. NewDynamoDBSessionRepositoryでDynamo DB用のセッション管理リポジトリを作ります
3. NewAuthHandlerの引数にセッションリポジトリを渡します
