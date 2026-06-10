.PHONY: build test lint clean

build:
	go build -o publish-policy-default ./cmd/module

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

clean:
	rm -f publish-policy-default
	rm -f cmd/module/module
