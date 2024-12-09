FROM golang:alpine as builder
ARG HTTP_PROXY=http://192.168.31.55:10809
ARG HTTPS_PROXY=http://192.168.31.55:10809
ARG GO111MODULE=on
ARG GOPROXY=https://goproxy.cn
RUN apk add --no-cache make git
WORKDIR /proxypool-src
COPY . /proxypool-src
RUN go mod download && \
    go mod tidy && \
    make docker && \
    mv ./bin/proxypool-docker /proxypool

FROM alpine:latest
ARG HTTP_PROXY=http://192.168.31.55:10809
ARG HTTPS_PROXY=http://192.168.31.55:10809
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /proxypool-src
COPY ./assets /proxypool-src/assets
COPY ./config /proxypool-src/config
COPY --from=builder /proxypool /proxypool-src/
ENTRYPOINT ["/proxypool-src/proxypool", "-d"]
