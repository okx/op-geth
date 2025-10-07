## run on local
```bash
make clean
cp local.env .env
./build_images.sh --all # build aggkit, cdk-erigon etc.
./1-pp-setup.sh
./3-deploy-op-contracts.sh
./4-stop-erigon.sh

# Build image.
docker build \
  --build-arg CHAIN_ID=195 \
  --progress=plain \
  -t op-migrate:latest -f dockerfile/Dockerfile.op-program .

# Run container 1 time and run steps 5-1, 5-2 and 5-3 (make reproducible-prestate) inside container.
# Exposing the Docker socket allows us to expose docker images to the container for `make reproducible-prestate`.
docker run \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$(pwd):/app/op-geth/test-pp-op" \
  -v "$(pwd)/data/cannon-data:/app/op-program/bin" \
  -e DOCKER_HOST=unix:///var/run/docker.sock \
  -d op-migrate:latest sleep infinity
# ssh into container.
docker exec -it ${CONTAINER_ID} /bin/bash

# Inside container (in test-pp-op directory)
cd /app/op-geth/test-pp-op
./5-1-migrate-prepare.sh
./5-2-migrate-op.sh
gzip -c merged.genesis.json > config-op/merged.genesis.gz.json
cp config-op/rollup.json /app/op-program/chainconfig/configs/195-rollup.json
cp config-op/merged.genesis.gz.json /app/op-program/chainconfig/configs/195-genesis-l2.json
cd /app
make reproducible-prestate

./6-start-op.sh
./7-setup-fraud-proof.sh
```

## run on testnet
```bash
# pause erigon, update .env fork_num
cp testnet.env .env
./3-deploy-op-contracts.sh

# LOCAL ENVIRONMENT
# ----------------------------------------------------------------------------

# Build the image locally after deploying contracts (rollup.json and genesis.json).
docker build \
  --platform linux/amd64 \
  --build-arg ENV=testnet \
  --build-arg CHAIN_ID=196 \
  --build-arg OP_STACK_IMAGE=op-stack:amd64 \
  --progress=plain \
  -t op-migrate:amd64 -f dockerfile/Dockerfile.op-program .

# 1) docker.io/library/golang:1.24.2-alpine3.21
docker pull golang@sha256:3077e12cda6debf8a9eba8eba0b6b4efe6f9c17295a18e3883cc5797d1688acb
docker tag 3077e12cd golang:1.24.2-alpine3.21
# 2) docker.io/library/golang:1.23.8-alpine3.21
docker pull golang@sha256:cc94cc0110a0ca83ec24991c0e981ca57f88a0bc959c7e7532701d6b1956668d
docker tag cc94cc01 golang:1.23.8-alpine3.21

docker save golang:1.24.2-alpine3.21 | gzip > golang-1.24.2-alpine3.21.tar.gz
docker save golang:1.23.8-alpine3.21 | gzip > golang-1.23.8-alpine3.21.tar.gz
docker save op-migrate:amd64 | gzip > op-migrate-amd64.tar.gz
docker save op-geth:7706694 | gzip > op-geth.tar.gz # starting new OP sequencer

# Make a new folder in current directory.
mkdir upload-to-ecs
mv golang-1.24.2-alpine3.21.tar.gz golang-1.23.8-alpine3.21.tar.gz op-migrate-amd64.tar.gz op-geth.tar.gz upload-to-ecs
tar -czf upload-to-ecs.tar.gz upload-to-ecs
# Manually copy upload-to-ecs.tar.gz to DACs env.

# INSIDE DACs TERMINAL
# ----------------------------------------------------------------------------

# Calculate md5 hash to create OSS ticket.
md5sum upload-to-ecs.tar.gz
# Use osstool to upload images to ECS. 
./osstool -f upload-to-ecs.tar.gz -a upload -ticket ${ticket-id}


# INSIDE ECS MACHINE
# ----------------------------------------------------------------------------
docker run \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "/data/test-pp-op:/app/op-geth/test-pp-op" \
  -v "/data/cannon-data:/app/op-program/bin" \
  -e DOCKER_HOST=unix:///var/run/docker.sock \
  -d op-migrate:amd64 sleep infinity
# ssh into container.
docker exec -it ${CONTAINER_ID} /bin/bash

# INSIDE CONTAINER 
# ----------------------------------------------------------------------------
cd /app/op-geth/test-pp-op
./5-1-migrate-prepare.sh
./5-2-migrate-op.sh
gzip -c merged.genesis.json > config-op/merged.genesis.gz.json
cp config-op/rollup.json /app/op-program/chainconfig/configs/196-rollup.json
cp config-op/merged.genesis.gz.json /app/op-program/chainconfig/configs/196-genesis-l2.json
cd /app
make reproducible-prestate

# Leave the container
exit

# OUTSIDE CONTAINER 
# ----------------------------------------------------------------------------
./6-start-op.sh
./7-setup-fraud-proof.sh
```

## Troubleshooting
1. if tls: failed to verify certificate: x509: certificate signed by unknown authority
need to use a different contracts image
```
FROM op-contracts:v1.13.4

# Update certificates
RUN apt-get update && apt-get install -y ca-certificates && update-ca-certificates

# Set environment variables for Go
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_DIR=/etc/ssl/certs
```

2. deploy contracts
update intent.toml, set chainId

3. make op-program
```
./5-3-build-op-program.sh -a x86
docker commit my-op-temp my-op-stack:built
```

