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
update intent.toml

3. make op-program
```
./5-3-build-op-program.sh -a x86
docker commit my-op-temp my-op-stack:built
```

