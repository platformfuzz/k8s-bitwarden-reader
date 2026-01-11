# Bitwarden Reader

A Go-based Kubernetes application for reading and displaying Bitwarden secrets synced to Kubernetes.

## Overview

Bitwarden Reader is a web application that reads and displays Bitwarden secrets that have been synced to Kubernetes. It provides a web UI and REST API for viewing secret information, sync status, and triggering manual syncs.

## Features

- **Web UI**: Modern, responsive interface for viewing secrets
- **REST API**: JSON API for programmatic access
- **Real-time Updates**: WebSocket support for live secret status updates
- **Sync Management**: Trigger manual syncs for Bitwarden secrets
- **Sync Status**: Display detailed sync information from CRDs
- **Standalone Mode**: Can run without Kubernetes access (limited features)

## Prerequisites

- Go 1.23 or later
- Kubernetes cluster access (optional - can run in standalone mode)
  - In-cluster: Automatic when running inside Kubernetes
  - Local: kubeconfig file at `~/.kube/config` or `KUBECONFIG` environment variable
- Docker (for containerized builds)

## Configuration

The application is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `POD_NAME` | Kubernetes pod name (from downward API) | - |
| `POD_NAMESPACE` | Kubernetes namespace (from downward API) | - |
| `SECRET_NAMES` | Comma-separated list of secret names to read | - |
| `APP_TITLE` | Application title | `Bitwarden Secrets Reader` |
| `APP_VERSION` | Application version | `1.0.0` |
| `DASHBOARD_REFRESH_INTERVAL` | WebSocket refresh interval in seconds | `5` |
| `SHOW_SECRET_VALUES` | Show secret values by default | `false` |

## Local Development

### Setup

1. Clone the repository
2. Download dependencies:
   ```bash
   make deps
   ```

### Running Locally

1. Set environment variables (optional):
   ```bash
   export POD_NAME=local-test
   export POD_NAMESPACE=default
   export SECRET_NAMES=bw-test-secret
   ```

2. Build and run:
   ```bash
   make build
   ./bin/bitwarden-reader
   ```

   Or run directly:
   ```bash
   go run ./cmd/server
   ```

3. Access the web UI at `http://localhost:8080`

**Note**: The application can run without Kubernetes access in standalone mode. In this mode:
- The web UI and API endpoints are still accessible
- Secret reading will show error messages indicating Kubernetes is unavailable
- Sync triggering will return 503 Service Unavailable
- Health endpoint works normally

## Building

### Build Go Binary

```bash
make build
```

The binary will be created at `bin/bitwarden-reader`.

### Build Docker Image

```bash
make docker-build
```

This creates a Docker image tagged as `bitwarden-reader:latest`.

### Run Docker Container

```bash
make docker-run
```

Or manually:
```bash
docker run -p 8080:8080 \
  -e POD_NAME=local-test \
  -e POD_NAMESPACE=default \
  -e SECRET_NAMES=bw-test-secret \
  bitwarden-reader:latest
```

## API Endpoints

### Web UI

- `GET /` - Web interface for viewing secrets

### REST API

- `GET /api/v1/secrets` - Get all secrets and sync information
  ```json
  {
    "secrets": [...],
    "namespace": "bitwarden-secrets",
    "totalFound": 2,
    "timestamp": "2026-01-11T12:00:00Z"
  }
  ```

- `POST /api/v1/trigger-sync` - Trigger manual sync for secrets
  ```json
  {
    "secretNames": ["bw-secret1", "bw-secret2"]
  }
  ```

- `GET /api/v1/health` - Health check endpoint
  ```json
  {
    "status": "healthy",
    "version": "1.0.0"
  }
  ```

### WebSocket

- `GET /ws` - WebSocket endpoint for real-time updates

## Project Structure

```
.
├── cmd/server/           # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── k8s/             # Kubernetes client operations
│   ├── reader/          # Core reading logic
│   └── server/          # HTTP server and handlers
├── web/
│   ├── static/          # Static assets (CSS, JS)
│   └── templates/       # HTML templates
├── Dockerfile           # Multi-stage Docker build
├── Makefile            # Build automation
└── README.md           # This file
```

## Kubernetes Deployment

### Standalone Mode vs Kubernetes Mode

The application supports two modes:

1. **Standalone Mode**: Runs without Kubernetes access
   - No kubeconfig or in-cluster config required
   - Limited functionality (UI accessible, but secrets cannot be read)
   - Useful for development and testing UI

2. **Kubernetes Mode**: Full functionality with Kubernetes access
   - Requires in-cluster config (when running in Kubernetes) or kubeconfig (local)
   - Full secret reading and sync management capabilities

### RBAC Requirements

When running in Kubernetes, the application requires the following RBAC permissions:

- `secrets`: `get`, `list`
- `bitwardensecrets` (CRD): `get`, `patch`

### Environment Variables in Kubernetes

Use Kubernetes downward API to inject pod information:

```yaml
env:
  - name: POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
  - name: POD_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
  - name: SECRET_NAMES
    value: "bw-secret1,bw-secret2"
```

## Testing

Run tests:
```bash
make test
```

Run code quality checks:
```bash
make lint  # Lint code
make fmt   # Format code
```

## Development Commands

- `make build` - Build Go binary
- `make test` - Run tests
- `make docker-build` - Build Docker image
- `make docker-run` - Run Docker container
- `make clean` - Clean build artifacts
- `make deps` - Download dependencies
- `make fmt` - Format code
- `make lint` - Lint code
- `make help` - Show all available commands

## License

MIT License - see LICENSE file for details
