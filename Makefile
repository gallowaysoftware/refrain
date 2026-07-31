.PHONY: build test vet lint fmt-check

build:
	go build -o bin/recall ./cmd/recall

test:
	go test -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

# CI fails if gofmt would change anything
fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "$$out"; exit 1; fi
