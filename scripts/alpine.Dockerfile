# Keep a current Go toolchain while exercising the upstream musl bundle's ICU 74 ABI.
FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS toolchain
FROM alpine:3.21@sha256:ce64758a109eb420d874a118f87920e625e12d3634e03b4a5573fd9f6e5d3507
COPY --from=toolchain /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:/usr/bin:/bin
RUN apk add --no-cache ca-certificates python3 gcc musl-dev libxml2 libstdc++ openssl readline krb5-libs zlib lz4-libs zstd-libs icu-libs \
    && adduser -D pgtest
