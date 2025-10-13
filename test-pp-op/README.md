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
  -v "$(pwd)/data/op-geth-seq:/app/op-geth/test-pp-op/data/op-geth-seq" \
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
exit # exit container

./6-start-op.sh
./7-setup-fraud-proof.sh
```

## run on testnet
```bash
cp testnet.env .env
./3-deploy-op-contracts.sh
# AFTER DEPLOYING OP CONTRACTS, CHECK TRANSACTOR ADDRESS ON SEPOLIA.
# NOTE: l1ProxyAdminOwner + opcm + transactor addr checking (check intent.toml)

# Update .env ()
# pause erigon, update .env fork_num.
Update FORK_BLOCK+1

# LOCAL ENVIRONMENT
# ----------------------------------------------------------------------------
# Build the image locally after deploying contracts (rollup.json and genesis.json).
docker build \
  --platform linux/amd64 \
  --build-arg CHAIN_ID=1952 \
  --build-arg OP_STACK_IMAGE=op-stack:amd64 \
  --progress=plain \
  -t op-migrate:amd64 -f dockerfile/Dockerfile.op-program .

# 1) docker.io/library/golang:1.24.2-alpine3.21
docker pull golang@sha256:3077e12cda6debf8a9eba8eba0b6b4efe6f9c17295a18e3883cc5797d1688acb
docker tag 3077e12cd golang:1.24.2-alpine3.21
# 2) docker.io/library/golang:1.23.8-alpine3.21
docker pull golang@sha256:b6da2ff7e4eb4c632f7f21532b775078f77a790b159c56a0a7963a1532364cf0
docker tag b6da2ff7e4e golang:1.23.8-alpine3.21

# From optimism root directory, build amd64 version of the golang base images above first.
docker build --platform linux/amd64 -t golang:1.23.8-alpine3.21-builder --build-arg GO_VERSION=1.23.8-alpine3.21 -f Dockerfile.repro-builder .
docker build --platform linux/amd64 -t golang:1.24.2-alpine3.21-builder --build-arg GO_VERSION=1.24.2-alpine3.21 -f Dockerfile.repro-builder .

docker save golang:1.24.2-alpine3.21-builder | gzip > golang-1.24.2-alpine3.21.tar.gz
docker save golang:1.23.8-alpine3.21-builder | gzip > golang-1.23.8-alpine3.21.tar.gz
docker save op-geth:v1.101511.0-patch | gzip > op-geth.tar.gz # starting new OP sequencer
docker save op-migrate:amd64 | gzip > op-migrate-amd64.tar.gz

# Make a new folder in current directory.
mkdir upload-to-ecs
mv golang-1.24.2-alpine3.21.tar.gz golang-1.23.8-alpine3.21.tar.gz op-migrate-amd64.tar.gz op-geth.tar.gz upload-to-ecs
tar -czvf upload-to-ecs.tar.gz upload-to-ecs
# Manually copy upload-to-ecs.tar.gz to DACs env.

# INSIDE DACs TERMINAL
# ----------------------------------------------------------------------------
# Calculate md5 hash to create OSS ticket.
md5sum upload-to-ecs.tar.gz
# Use osstool to upload images to ECS. 
./osstool -f upload-to-ecs.tar.gz -a upload -ticket ${ticket-id}

# INSIDE ECS MACHINE
# ----------------------------------------------------------------------------
# If not mounted memory, do this ONCE.
mkdir -p /mnt/ramdisk_op
mount -t tmpfs -o size=128g tmpfs /mnt/ramdisk_op
df -hT /mnt/ramdisk_op

# In disk
cd /data
# download from OSS
osstool download -ticket ${ticket-id} 
# untar the uploaded file
tar -xzvf upload-to-ecs.tar.gz
cd upload-to-ecs
# load the docker images into local registry
docker load < [filename].tar.gz
# Retag golang builder images as base golang images. During `make reproducible-prestate`,
# this will use the cached images instead of pulling from internet.
docker tag golang:1.23.8-alpine3.21-builder golang:1.23.8-alpine3.21
docker tag golang:1.24.2-alpine3.21-builder golang:1.24.2-alpine3.21


# START REGENESIS (ECS host machine)
# ----------------------------------------------------------------------------
docker run --rm -v /mnt/ramdisk_op:/mnt/ramdisk_op op-migrate:amd64 cp -rfv \
    /app/op-geth/test-pp-op/{5-all.sh,.env} \
    /mnt/ramdisk_op/test-pp-op

# All configs (including .env, op-geth-data, cannon-data) should be copied to this location.
cd /mnt/ramdisk_op/test-pp-op

# Execute all stage 5 in one step.
./5-all.sh

# RPC (init) since we couldn't start RPC with custom block.
# Once init, start the RPC (geth + node), it will take some time before it starts
# syncing new blocks from sequencer.
docker run --rm \
  -v "$(pwd):/app" \
  -v "$(pwd)/data/op-geth-rpc:/datadir" \
  op-geth:v1.101511.0-patch \
  geth \
  --datadir "/datadir" \
  --gcmode=archive \
  --db.engine=pebble \
  --log.format json \
  init \
  --state.scheme=hash \
  /app/merged.genesis.json 2>&1 | tee init.log

# start OP services
./6-start-op.sh # docker compose + .env
./7-setup-fraud-proof.sh # needs cast, docker compose
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

