# SPIFFE JWT-SVID caller -> API Gateway PoC (service-to-service)

This stack simulates a **service** ("caller") that:
1) is **attested** by a local SPIRE Agent (unix workload API socket),
2) fetches a **JWT-SVID** with a configured audience,
3) calls your **API Gateway** using `Authorization: Bearer <jwt-svid>`.

## Prereqs
- A reachable SPIRE Server (agent -> server TCP)
- A join token for the agent
- A registration entry for the caller workload

## Configure
Edit `agent/agent.conf`:
- `<SPIRE_SERVER_IP>` and `<JOIN_TOKEN>`

Edit `docker-compose.yml`:
- `API_URL` = your API Gateway invoke URL (stage + path)
- `AUDIENCE` must match your authorizer/extension `EXPECTED_AUDIENCE` (e.g., `api://poc`)

## Run
```bash
docker compose up -d --build
docker logs -f spiffe-caller
```

## One-time SPIRE server setup (registration entry)
Create an entry for this caller (PoC selector uses UID 10001; matches Dockerfile user):

```bash
spire-server entry create \
  -spiffeID spiffe://company.internal/ns/poc/sa/caller \
  -parentID spiffe://company.internal/spire/agent/poc-agent \
  -selector unix:uid:10001
```

(Adjust trust domain / IDs to your environment.)
