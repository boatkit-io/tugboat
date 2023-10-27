.PHONY: test
test:
	@go test ./... -coverprofile=c.out

.PHONY: lint
lint:
	@golangci-lint run
	@go run github.com/getoutreach/lintroller/cmd/lintroller@v1.17.0 -config lintroller.yaml ./...
