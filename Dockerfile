FROM docker.io/golang:1.19 AS builder

WORKDIR /workspace
COPY cmd/ cmd/
COPY hack/ hack/
COPY mock/ mock/
COPY pkg/ pkg/
COPY main.go .
COPY README.md README.md
COPY go.mod go.mod
COPY go.sum go.sum
RUN CGO_ENABLED=0 go build -ldflags "-w -s" -o /usr/local/bin/hd .

FROM alpine:3.10

COPY --from=builder /usr/local/bin/hd /usr/local/bin/hd
RUN hd fetch

CMD ["hd"]
