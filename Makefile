test:
	go test ./...

test-race:
	go test -race ./...

run:
	go run ./cmd/screensift -root ./screenshots
