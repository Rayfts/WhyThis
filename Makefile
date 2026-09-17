.PHONY: size-check fmt vet test race bench build vuln
fmt:
	gofmt -w ./cmd ./internal ./pkg
vet:
	go vet ./...
test:
	go test ./...
race:
	go test -race ./...
bench:
	go test -run '^$$' -bench . -benchmem ./internal/gitx ./internal/history ./internal/index ./internal/risk ./internal/storage
build:
	go build ./cmd/whythis
vuln:
	govulncheck ./...

size-check:
	./scripts/check-go-file-size.sh
