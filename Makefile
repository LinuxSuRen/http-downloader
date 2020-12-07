build: fmt
	CGO_ENABLE=0 go build -ldflags "-w -s" -o bin/github-go

run:
	go run main.go

fmt:
	go fmt ./...

copy: build
	sudo cp bin/github-go /usr/local/bin/github-go
