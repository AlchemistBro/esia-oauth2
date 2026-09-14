.PHONY: fmt test race vet build check vuln

fmt:
	gofmt -w *.go

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

build:
	CGO_ENABLED=0 go build -o callback-server .

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

check:
	test -z "$$(gofmt -l *.go)"
	go test ./...
	go test -race ./...
	go vet ./...
	CGO_ENABLED=0 go build -o /tmp/callback-server .
