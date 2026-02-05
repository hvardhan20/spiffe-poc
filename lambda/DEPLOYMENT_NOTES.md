# Deployment Notes (PoC)

## Build zips
From repo root:
```bash
chmod +x scripts/*.sh
./scripts/build_all.sh
```

Produces:
- lambda/dist/extension-layer.zip
- lambda/dist/authorizer.zip
- lambda/dist/business.zip

## Lambda runtimes
These builds produce a `bootstrap` binary (custom runtime format). When you create the Lambda:
- Runtime: **provided.al2** (or provided.al2023 if you prefer and adjust)
- Handler: ignored for custom runtime (still set something like `bootstrap`)

## Attach extension layer to the Authorizer Lambda
Create a Lambda Layer from `extension-layer.zip` and attach it to the authorizer function.

## Environment variables (Authorizer Lambda)
These env vars are read by the extension process:
- OIDC_ISSUER_URL=http://<EC2_PRIVATE_IP>
- EXPECTED_AUDIENCE=api://poc
- REQUIRED_TRUST_DOMAIN=spiffe://company.internal
- JWKS_REFRESH=120s

(Optionally) set:
- LISTEN_ADDR=127.0.0.1:2773
- HTTP_TIMEOUT=2s

## API Gateway (REST API)
- Create a REST API + resource + method (e.g., GET /hello)
- Configure Lambda proxy integration to `business` lambda
- Create a REQUEST Lambda authorizer pointing to the `authorizer` lambda
  - identity source: method.request.header.Authorization
- Attach the authorizer to the method

## Passing principalId to backend
If you use proxy integration, authorizer context often appears in:
- event.requestContext.authorizer

Optionally, you can map it to a header:
- X-Spiffe-Id: $context.authorizer.principalId
