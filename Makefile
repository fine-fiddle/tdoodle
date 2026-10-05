.PHONY: build test check

build:
	go build -o tdoodle .

test:
	go test ./...

check:
	go vet ./...
	go test -race ./...
