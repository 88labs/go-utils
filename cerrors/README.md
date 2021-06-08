## cerrors

ANDPADの標準的なエラー型です。xerrorsを使っています。

### Usage

**エラーの生成**

```go
cerrors.New(aerrors.UnknownErr, err, "detail")
```

書式指定ができるNewfもあります。

**エラーのラッピング**

```go
xerrors.Errorf(": %w", err)
```
