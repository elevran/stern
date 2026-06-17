BINARY := stern
BUILD_DIR := bin
MODULE := github.com/elevran/stern

.PHONY: build test lint tidy clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/stern/...

test:
	go test -race -count=1 ./...

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not found, run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)
