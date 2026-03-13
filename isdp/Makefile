.PHONY: build run test clean docker-up docker-down

build:
	go build -o bin/isdp ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v -cover

clean:
	rm -rf bin/

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down