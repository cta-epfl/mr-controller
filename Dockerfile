# helm: .containers.main
FROM golang:1.21.1-alpine3.18 as builder

ARG GOPROXY

RUN apk add --no-cache git && rm -rf /var/cache/apk/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

WORKDIR /app
RUN export LOCAL_DEV=NOT
RUN CGO_ENABLED=0 go build -o service

FROM golang:1.21.1-alpine3.18 AS final

WORKDIR /app
RUN apk add --no-cache git && rm -rf /var/cache/apk/*
COPY --from=builder /app/service .

CMD ["./service"]


# FROM debian:bookworm-slim

# LABEL author=ph0tonic
# ENV UID=1000

# RUN apt-get update && apt-get install -y \
#     git \
#     curl \
#     bash \
#   && rm -rf /var/lib/apt/lists/*

# RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
#   && install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl \
#   && rm kubectl

# RUN [ -x /usr/sbin/useradd ] && useradd -m -u ${UID} ephemeral-controller -s /bin/bash

# USER ${UID}}

# ENTRYPOINT [ "bash" ]