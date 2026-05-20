VERSION ?= 0.2.0
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

.PHONY: build test clean release tidy

build:
	go build $(LDFLAGS) -o bin/kunjani .

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/ dist/

release:
	rm -rf dist
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/kunjani-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/kunjani-darwin-arm64 .
	GOOS=linux  GOARCH=amd64 go build $(LDFLAGS) -o dist/kunjani-linux-amd64  .
