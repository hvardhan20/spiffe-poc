FROM ghcr.io/spiffe/spire-agent:1.10.0 AS spire

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl bash jq

# Copy spire-agent CLI (includes `api fetch jwt`)
COPY --from=spire /opt/spire/bin/spire-agent /usr/local/bin/spire-agent

# Create a non-root user; we'll register selector unix:uid:10001
RUN adduser -D -u 10001 appuser
USER 10001

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
