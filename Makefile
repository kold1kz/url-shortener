cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out

test:
	go test -v ./...

build:
	go build -o shortener ./cmd/shortener/main.go
	./shortenertest -test.v -test.run=^TestIteration1$ -binary-path=cmd/shortener/shortener

check:
	goimports -l .
	gofmt -l .
	go test -v ./...

fix_check:
	goimports -w .
	gofmt -w .

staticlint:
	go run ./cmd/staticlint ./...

reset:
	go run ./cmd/reset