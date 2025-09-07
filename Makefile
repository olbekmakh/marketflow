.PHONY: build run test clean docker-setup docker-clean

build:
	go build -o marketflow .

run: build
	./marketflow

test:
	go test ./...

clean:
	rm -f marketflow

install-deps:
	go mod tidy

docker-setup:
	docker run --name postgres-marketflow -e POSTGRES_USER=marketflow -e POSTGRES_PASSWORD=password -e POSTGRES_DB=marketflow -p 5432:5432 -d postgres:15
	docker run --name redis-marketflow -p 6379:6379 -d redis:7

docker-clean:
	docker stop postgres-marketflow redis-marketflow || true
	docker rm postgres-marketflow redis-marketflow || true
EOF