# Keep a current Go toolchain while exercising the upstream musl bundle's ICU 74 ABI.
FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS toolchain
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
COPY --from=toolchain /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN apk add --no-cache ca-certificates python3 gcc musl-dev libxml2 libstdc++ openssl readline krb5-libs zlib lz4-libs zstd-libs icu-libs \
    && adduser -D pgtest
