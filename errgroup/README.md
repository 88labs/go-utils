# errgroup

このerrgroupは、golang.org/x/sync/errgroupをベースとしています。
errgroupで実行したgoroutine内でpanicが起きた場合、Group.Wait()でpanicが発生する点が違いです。
これにより、goroutineを呼び出した側でrecoverすることができます。
