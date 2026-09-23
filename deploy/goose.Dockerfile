FROM alpine:3.20

ARG GOOSE_VERSION=3.27.2

RUN apk add --no-cache ca-certificates curl \
 && curl -fsSL "https://github.com/pressly/goose/releases/download/v${GOOSE_VERSION}/goose_linux_x86_64" \
      -o /usr/local/bin/goose \
 && chmod +x /usr/local/bin/goose

COPY migrations /migrations

WORKDIR /migrations

ENTRYPOINT ["goose"]