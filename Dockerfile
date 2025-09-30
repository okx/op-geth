# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

# Build Geth in a stock Go builder container
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev linux-headers git

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-ethereum/
COPY go.sum /go-ethereum/
RUN cd /go-ethereum && go mod download

ADD . /go-ethereum
# Build both geth and devp2p tools
RUN cd /go-ethereum && go run build/ci.go install -static ./cmd/geth ./cmd/devp2p

# Pull Geth into a second stage deploy alpine container
FROM alpine:latest AS base

RUN apk add --no-cache ca-certificates

# Add some metadata labels to help programmatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""
LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"

FROM base AS geth
COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/
EXPOSE 8545 8546 30303 30303/udp
ENTRYPOINT ["geth"]

FROM base AS devp2p
COPY --from=builder /go-ethereum/build/bin/devp2p /usr/local/bin/
EXPOSE 30303 30303/udp
ENTRYPOINT ["devp2p"]
