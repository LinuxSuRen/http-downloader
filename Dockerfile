FROM alpine:3.10

COPY hd /usr/local/bin/hd
RUN hd fetch

CMD ["hd"]
