cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out

test:
	go test -v ./...

build:
	go build -o shortener *.go
	./shortenertest -test.v -test.run=^TestIteration1$ -binary-path=cmd/shortener/shortener