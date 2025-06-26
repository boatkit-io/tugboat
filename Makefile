.PHONY: test
test:
	@go test ./... -coverprofile=c.out

.PHONY: lint
lint:
	@golangci-lint run
	@go run github.com/getoutreach/lintroller/cmd/lintroller@v1.18.8 -config lintroller.yaml ./...

.PHONY: generate
generate:
	@go generate ./...
