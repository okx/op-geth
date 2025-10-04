## run on local
```
make clean
cp local.env .env
./1-pp-setup.sh
./3-deploy-op-contracts.sh
./4-stop-erigon.sh
./5-1-migrate-prepare.sh
./5-2-migrate-op.sh
./5-3-build-op-program.sh
./6-start-op.sh
./7-setup-fraud-proof.sh
```

## run on testnet
```
./3-deploy-op-contracts.sh
# pause erigon, update .env fork_num
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

