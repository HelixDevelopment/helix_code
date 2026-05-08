FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o /build/helixcode ./cmd/server/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S helixcode && adduser -S -G helixcode helixcode

COPY --from=builder /build/helixcode /usr/local/bin/helixcode
COPY config/config.yaml /etc/helixcode/config.yaml

RUN mkdir -p /var/lib/helixcode && chown -R helixcode:helixcode /var/lib/helixcode

USER helixcode
EXPOSE 8080
VOLUME ["/var/lib/helixcode", "/etc/helixcode"]

ENTRYPOINT ["helixcode"]
CMD ["server"]
