## cerrors

よく使いそうなエラー種類を定義しました。xerrorsを使っています。

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
