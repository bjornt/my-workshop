BINARY := my-workshop

.PHONY: build test fmt vet check clean

build:
	go build -o $(BINARY) ./cmd/my-workshop

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check: fmt vet test

clean:
	rm -f $(BINARY)
	rm -rf dist
