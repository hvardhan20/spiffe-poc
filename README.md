# SPIFFE -> API Gateway -> Lambda Authorizer -> Lambda (PoC)

This repo is a **from-scratch PoC** showing how to authenticate inbound requests to Lambda using **SPIFFE JWT-SVIDs**
(via API Gateway + Lambda REQUEST Authorizer), accelerated by a **Lambda Extension** that caches OIDC discovery/JWKS
and performs local verification.

## Why JWT-SVID (not mTLS) for inbound to Lambda?
When you use API Gateway -> Lambda, the Lambda runtime never sees the TLS handshake, so you can't do true inbound mTLS
auth to Lambda. Instead, we use **SPIFFE JWT-SVID** as the inbound credential and verify it in the authorizer.

## Contents
- `docker-compose.yml` : runs Postgres + SPIRE Server + SPIRE OIDC discovery provider on a single EC2 host
- `spire/config/` : SPIRE server + OIDC provider configs
- `lambda/extension/` : Go Lambda extension (local verifier, JWKS cache)
- `lambda/authorizer/` : Go API Gateway REQUEST authorizer
- `lambda/business/` : sample business lambda returning the authenticated SPIFFE ID
- `scripts/` : build scripts for Lambda zips and extension layer

## Quick start (EC2 Docker)
1. Copy this repo to an EC2 instance inside your company VPC (private subnet recommended).
2. Install Docker + docker-compose plugin.
3. Edit the issuer in `spire/config/oidc.conf` to the EC2 private IP:
   - `issuer = "http://<YOUR_EC2_PRIVATE_IP>"`
4. Start SPIRE + OIDC + Postgres:
   ```bash
   docker compose up -d
   ```
5. Validate:
   ```bash
   curl http://127.0.0.1/.well-known/openid-configuration
   curl http://127.0.0.1/keys
   ```

## Lambda deployment overview (AWS Console / IaC)
- API Gateway: **REST API**, method uses **REQUEST Lambda authorizer**
- Lambdas run **in the same VPC** and must be able to reach `http://<EC2_PRIVATE_IP>/...` (OIDC discovery + JWKS).
- Attach the extension as a **Lambda Layer** to the authorizer Lambda.

### Extension env vars (set on Authorizer Lambda)
- `OIDC_ISSUER_URL=http://<EC2_PRIVATE_IP>`
- `EXPECTED_AUDIENCE=api://poc`
- `REQUIRED_TRUST_DOMAIN=spiffe://company.internal`
- `JWKS_REFRESH=120s`

## Build artifacts
From your laptop (or CI), build zips:
```bash
cd lambda
../scripts/build_all.sh
```

This produces:
- `dist/authorizer.zip`
- `dist/business.zip`
- `dist/extension-layer.zip` (contains `/opt/extensions/spiffe-verifier-extension`)

Upload zips to Lambda / Layer.

## Notes on callers
To produce a real SPIFFE JWT-SVID, run a caller workload with a SPIRE Agent (EKS or ECS on EC2), request a JWT-SVID with:
- audience: `api://poc`
Then call API Gateway with:
- `Authorization: Bearer <jwt-svid>`
