cover:
	go test -coverprofile=coverage.out $(shell go list ./... | grep -v -E '(/proto|/mocks|/cmd/staticlint|/cmd/workload)')
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

proto:
	protoc \
      -I . \
      --go_out=. --go_opt=paths=source_relative \
      --go-grpc_out=. --go-grpc_opt=paths=source_relative \
      --go_opt=default_api_level=API_OPAQUE \
      proto/shortener.proto
