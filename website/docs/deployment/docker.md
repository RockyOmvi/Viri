# Docker Deployment

## Architecture

The Docker Compose setup creates 5 services:

- **bootstrap**: Entry node for peer discovery
- **validator-0** to **validator-3**: Consensus validators

## Configuration

Each service uses:
- `Dockerfile` for the Go binary
- `testnet/configs/validator-N/config.json` for node configuration
- `testnet/keys/validator-N.key` for secp256k1 private keys
- Shared genesis via `testnet/genesis/genesis.json`

## Build

```bash
# Build images (use --no-cache when source changes)
docker compose build --no-cache

# Start all services
docker compose up -d

# View logs
docker compose logs -f bootstrap

# Stop all
docker compose down

# Reset data
docker compose down -v
```

## Customization

Edit `docker-compose.yml` to:
- Change port mappings
- Adjust resource limits
- Add more validators
- Modify environment variables
