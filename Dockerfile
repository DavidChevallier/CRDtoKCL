FROM golang:1.22.4-alpine3.20 AS builder
RUN apk add --no-cache bash git \
    && wget -q https://kcl-lang.io/script/install-cli.sh -O - | /bin/bash
WORKDIR /app
COPY . .
RUN go build -o crdtokcl main.go

FROM alpine:latest
RUN apk add --no-cache bash
WORKDIR /app
COPY --from=builder /app/crdtokcl /bin/crdtokcl
COPY --from=builder /usr/local/bin/kcl /usr/local/bin/kcl
ENTRYPOINT ["/bin/crdtokcl"]