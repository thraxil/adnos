BINARY_NAME=adnos

all: build test

build:
	go build -o $(BINARY_NAME) main.go

test:
	go test -v ./...

clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -rf data/

run:
	go run main.go

.PHONY: all build test clean run
