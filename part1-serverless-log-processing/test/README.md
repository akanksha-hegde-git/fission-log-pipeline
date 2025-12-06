# Testing Guide

Basic tests for the log transformer function.

## Quick Test
```bash
./test-single.sh
```

This runs 4 tests:
1. ERROR log with all fields
2. WARN log (lowercase level)
3. INFO log without timestamp
4. Abbreviated level (ERR → ERROR)

## Manual Testing

### Basic Request
```bash
curl -X POST http://$FISSION_ROUTER/transform-logs \
  -H "Content-Type: application/json" \
  -d '{"level":"ERROR","service":"test","message":"Hello Fission!"}'
```

## Test Data

Sample logs are in `sample-logs.json`. Use them like:
```bash
# Test single error log
curl -X POST http://$FISSION_ROUTER/transform-logs \
  -H "Content-Type: application/json" \
  -d "$(jq '.single_error' sample-logs.json)"

# Test batch
curl -X POST http://$FISSION_ROUTER/transform-logs \
  -H "Content-Type: application/json" \
  -d "$(jq '.batch_mixed' sample-logs.json)"
```

## Troubleshooting

### Router not accessible
```bash
# Verify router URL
echo $FISSION_ROUTER

# Set it if empty
export FISSION_ROUTER=$(minikube ip):$(kubectl get svc router -n fission -o jsonpath='{.spec.ports[0].nodePort}')
```

### Function not responding
```bash
# Check if deployed
fission function list

# Check pods
kubectl get pods -n fission-function

# View logs
fission function logs --name log-transformer
```