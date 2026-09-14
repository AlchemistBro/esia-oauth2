# ESIA OAuth 2.0 Callback Service

Small Go service implementing the server side of an ESIA OpenID Connect flow
with CryptoPro-backed GOST signatures.

> Independent integration example. Not an official ESIA or CryptoPro product.

## What it does

- creates a browser-bound, single-use OAuth `state`;
- signs authorization requests through local CryptoPro;
- exchanges the authorization code on the server;
- verifies GOST 2012 or RSA ID token signatures;
- validates `alg`, `typ`, `sbt == "id"`, `iss`, `aud`, `sub`, `exp`, `nbf`
  and `iat`;
- never sends access or ID tokens to the browser.

## Tests

Go 1.25.14 or newer is required. The tests use fakes and do not need an ESIA
account, CryptoPro installation or a real configuration file.

```bash
test -z "$(gofmt -l *.go)"
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

## Configuration

Copy `config.example.json` to `config.json` and replace every placeholder.
Keep the real config, PIN, key container and certificates outside version
control.

OAuth state is stored in memory, so the service is intended for one replica.
A multi-replica deployment needs a shared atomic state store.

## Docker

The runtime image requires licensed CryptoPro Linux `.deb` packages. Create
`deploy/cryptopro/`, place the required packages there and build the image:

```bash
docker build -t esia-callback:local .
```

Run the service behind a TLS reverse proxy and do not publish its application
port directly. CryptoPro packages, certificates and key containers are
deployment inputs and are intentionally absent from this repository.
