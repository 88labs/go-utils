# go-utils



## Testing
- required [Task](https://github.com/go-task/task)
- test all packages
```shell
task -p test
```

## Import format
- required [Task](https://github.com/go-task/task)
- install go-oif
```shell
curl -sSfL https://raw.githubusercontent.com/heyvito/go-oif/main/install.sh | sh -s -- -b $(go env GOPATH)/bin
```
- run
```shell
task -p go-oif
```

## Go modules
- required [Task](https://github.com/go-task/task)
- go mod tidy all packages
```shell
task -p go-mod-tidy
```
