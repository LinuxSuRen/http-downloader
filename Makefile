build: fmt test
	export GOPROXY=https://goproxy.io
	CGO_ENABLE=0 go build -ldflags "-w -s" -o bin/hd

build-linux: fmt lint
	export GOPROXY=https://goproxy.io
	CGO_ENABLE=0 GOOS=linux go build -ldflags "-w -s" -o bin/linux/hd
	upx bin/linux/hd

test: fmt
	go test ./... -coverprofile coverage.out

run:
	go run main.go

fmt:
	go fmt ./...

lint:
	golint -set_exit_status=1 ./...

copy: build
	sudo cp bin/hd /usr/local/bin/

init: gen-mock
gen-mock:
	go get github.com/golang/mock/gomock
	go install github.com/golang/mock/mockgen
	mockgen -destination ./mock/mhttp/roundtripper.go -package mhttp net/http RoundTripper

update:
	git fetch
	git reset --hard origin/$(shell git branch --show-current)