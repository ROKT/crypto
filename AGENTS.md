# crypto

## Overview

Rokt's fork of `golang.org/x/crypto` — the official Go supplementary cryptography packages. This repository tracks the upstream `golang/crypto` repository and may contain Rokt-specific patches.

## Tech Stack

- **Language:** Go (1.25.0)
- **Module:** `golang.org/x/crypto`
- **Upstream:** `github.com/golang/crypto` (configured as `upstream` remote)

## Repository Structure

This follows the standard `golang.org/x/crypto` layout. Key packages include:

```
ssh/                 # SSH client/server implementation
acme/                # ACME protocol (Let's Encrypt)
bcrypt/              # bcrypt password hashing
argon2/              # Argon2 key derivation
chacha20poly1305/    # ChaCha20-Poly1305 AEAD
curve25519/          # Curve25519 Diffie-Hellman
ed25519/             # Ed25519 signatures
hkdf/                # HMAC-based key derivation
nacl/                # NaCl cryptographic library
openpgp/             # OpenPGP message format
pkcs12/              # PKCS#12 key stores
sha3/                # SHA-3 hash functions
tls (via acme/)      # TLS certificate management
```

## Important Notes

- This is a **fork** of the Go standard library supplementary crypto package.
- The `upstream` remote points to `github.com/golang/crypto`.
- Changes should be minimal and Rokt-specific; upstream syncs may overwrite modifications.
- There is no Makefile or Justfile — use standard `go` tooling (`go build`, `go test`).

## Default Branch

`master`
