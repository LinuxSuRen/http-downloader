build: fmt
	CGO_ENABLE=0 go build -ldflags "-w -s" -o bin/hd

run:
	go run main.go

fmt:
	go fmt ./...

copy: build
	sudo cp bin/hd /usr/local/bin/
