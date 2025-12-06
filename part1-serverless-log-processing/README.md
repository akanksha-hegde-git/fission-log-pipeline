# Part 1: Serverless Log Processing with Fission + Go
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.20+-blue.svg)](https://go.dev)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.27+-326CE5.svg)](https://kubernetes.io)
[![Fission](https://img.shields.io/badge/Fission-1.21+-orange.svg)](https://fission.io)

A complete, production-ready serverless log processing pipeline built with Fission, Go, and Kubernetes.

## What You'll Build

A serverless log transformer fission function that:
- Processes 1000+ logs/second
- 120ms cold starts
- Auto-scales based on load
- Runs on any Kubernetes cluster
- Zero vendor lock-in

## Architecture
```
         ┌─────────────┐
         │  Services   │ (Microservices logging)
         └──────┬──────┘
                │ HTTP POST
                ▼
┌─────────────────────────────────┐
│    Kubernetes (Minikube)        │
│  ┌───────────────────────────┐  │
│  │  Fission Router           │  │
│  └────────────┬──────────────┘  │
│               ▼                 │
│  ┌───────────────────────────┐  │
│  │  Log Transformer (Go)     │  │
│  │  • Normalize formats      │  │
│  │  • Enrich metadata        │  │
│  │  • Aggregate metrics      │  │
│  └───────────────────────────┘  │
└─────────────────────────────────┘
                │
                ▼
        ┌──────────────────┐
        │ Transformed JSON │
        └──────────────────┘
```

### Prerequisites
- Minikube (v1.30+)
- kubectl (v1.27+)
- Docker Desktop
- Helm 3

**Suggested System Requirements:**
Not mandatory but recommended for better performance,
- 4 CPU cores
- 8GB RAM
- 20GB disk space

## What This Does

Transforms inconsistent log formats into standardized JSON:

**Input (various formats):**
```json
{"lvl":"ERR","msg":"DB timeout","ts":1730715600}
{"level":"error","message":"failed","timestamp":"2024-11-04T10:00:00Z"}
{"severity":"E","text":"down"}
```

**Output (standardized):**
```json
{
  "transformed_logs": [{
    "level": "ERROR",
    "message": "DB timeout",
    "service": "auth-service",
    "timestamp_iso": "2024-11-04T05:00:00Z",
    "received_at": "2024-11-27T10:30:00Z",
    "request_id": "req-abc123",
    "pipeline_stage": "fission-log-processor"
  }],
  "metrics": {
    "total_logs": 1,
    "errors_by_service": {"auth-service": 1},
    "errors_by_type": {"db_timeout": 1}
  }
}
```

## Quick Start

### Prerequisites
- Minikube running
- Fission installed  
- Docker Desktop running

## Setup Environment:
```bash
cd part-1-serverless-processing
./scripts/setup-minikube.sh
./scripts/install-fission.sh
```

## Manual Deployment

If you prefer manual steps:
```bash
# 1. Build Docker image (for Minikube)
eval $(minikube docker-env)
docker build -t log-transformer:v1 .

# OR for Docker Hub
export DOCKER_USERNAME="your-username"
docker build -t $DOCKER_USERNAME/log-transformer:v1 .
docker push $DOCKER_USERNAME/log-transformer:v1

# 2. Deploy function
fission function run-container \
  --name log-transformer \
  --image log-transformer:v1 \
  --port 8888 \
  --minscale 1 \
  --maxscale 3

# 3. Create route
fission route create \
  --name transform-logs-route \
  --method POST \
  --url /transform-logs \
  --function log-transformer

# 4. Test
export FISSION_ROUTER=$(minikube ip):$(kubectl get svc router -n fission -o jsonpath='{.spec.ports[0].nodePort}')
curl -X POST http://$FISSION_ROUTER/transform-logs \
  -H "Content-Type: application/json" \
  -d '{"level":"ERROR","service":"test","message":"Hello!"}'
```

## Testing
```bash
# Basic functionality
./test/test-single.sh
```

## Troubleshooting

### Function not starting?
```bash
kubectl get pods -n fission-function
kubectl logs -n fission-function <pod-name>
```

### Can't access router?
```bash
export FISSION_ROUTER=$(minikube ip):$(kubectl get svc router -n fission -o jsonpath='{.spec.ports[0].nodePort}')
echo $FISSION_ROUTER
```

### Image pull errors?
```bash
# Use Minikube's Docker daemon
eval $(minikube docker-env)
docker build -t log-transformer:v1 .
```

## Documentation

- **[Deployment Guide](deploy/README.md)** - Deploy instructions
- **[Testing Guide](test/README.md)** - Test scripts
- **[Troubleshooting](#troubleshooting)** - Common issues
