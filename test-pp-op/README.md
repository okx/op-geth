## run on local
```
make clean
cp local.env .env
./build_images.sh --all # build aggkit, cdk-erigon etc.
./1-pp-setup.sh
./3-deploy-op-contracts.sh
./4-stop-erigon.sh

# Build image.
docker build -t op-migrate:latest --progress=plain -f dockerfile/Dockerfile.op-program .

# Run container 1 time and run steps 5-1, 5-2 and 5-3 (make reproducible-prestate) inside container.
# Exposing the Docker socket allows us to expose docker images to the container for `make reproducible-prestate`.
docker run \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$(pwd)/data/cannon-data:/app/op-program/bin" \
  -v "$(pwd)/data/:/app/op-geth/test-pp-op/data" \
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
make reproducible-prestate

./6-start-op.sh
./7-setup-fraud-proof.sh
```

## run on testnet
```
# Upload these images to ECS machine to ensure it is not downloaded from internet.
# docker.io/library/golang:1.24.2-alpine3.21
# docker.io/library/golang:1.23.8-alpine3.21

./3-deploy-op-contracts.sh
# pause erigon, update .env fork_num
# To be updated...
./5-1-migrate-prepare.sh
./5-2-migrate-op.sh
./5-3-build-op-program.sh
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

