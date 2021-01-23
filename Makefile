build: fmt
	CGO_ENABLE=0 go build -ldflags "-w -s" -o bin/hd

run:
	go run main.go

fmt:
	go fmt ./...

copy: build
	sudo cp bin/hd /usr/local/bin/

init: gen-mock
gen-mock:
	go get github.com/golang/mock/gomock
	go install github.com/golang/mock/mockgen
	mockgen -destination ./mock/mhttp/roundtripper.go -package mhttp net/http RoundTripper
