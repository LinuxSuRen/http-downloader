build: fmt test
	export GOPROXY=https://goproxy.io
	CGO_ENABLED=0 go build -ldflags "-w -s" -o bin/hd

build-windows:
	GOOS=windows CGO_ENABLED=0 go build -ldflags "-w -s" -o bin/windows/hd.exe
build-linux: fmt lint build-linux-no-check
build-linux-no-check:
	export GOPROXY=https://goproxy.io
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -o bin/linux/hd
	upx bin/linux/hd

test: fmt
	go test ./... -coverprofile coverage.out
pre-commit: fmt test build
cp-pre-commit:
	cp .github/pre-commit .git/hooks/pre-commit
run:
	go run main.go

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

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
goreleaser:
	goreleaser build --snapshot --rm-dist
