FROM alpine:3.10

RUN apk add --upgrade apk-tools@2.10.7-r0
COPY hd /usr/local/bin/hd
RUN hd fetch

CMD ["hd"]
