ARG GO_VERSION=1.27
FROM golang:${GO_VERSION}-alpine
RUN apk add --no-cache python3 git github-cli ca-certificates \
    && adduser -D consumer
WORKDIR /consumer
COPY scripts/check-published.py scripts/check-published.py
COPY scripts/testdata/published scripts/testdata/published
USER consumer
ENTRYPOINT ["python3", "scripts/check-published.py"]
