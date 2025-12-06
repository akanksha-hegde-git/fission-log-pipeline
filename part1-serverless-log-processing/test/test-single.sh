#!/bin/bash

echo "╔════════════════════════════════════════════════════════╗"
echo "║      Log Transformer - Single Log Test                 ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Get router URL
if command -v minikube &> /dev/null; then
    FISSION_ROUTER=$(minikube ip):$(kubectl get svc router -n fission -o jsonpath='{.spec.ports[0].nodePort}')
else
    FISSION_ROUTER=$(kubectl get svc router -n fission -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
fi

echo -e "${BLUE}Router URL:${NC} $FISSION_ROUTER"
echo ""

# Test 1: Single ERROR log
echo -e "${YELLOW}Test 1: Single ERROR log${NC}"
echo "Request:"
cat << 'EOF' | jq '.'
{
  "level": "ERROR",
  "timestamp": 1730715600,
  "service": "auth-service",
  "message": "Database connection timeout",
  "error_type": "db_timeout",
  "request_id": "req-101"
}
EOF

echo ""
echo "Response:"
curl -s -X POST http://$FISSION_ROUTER/transform-logs \
  -H "Content-Type: application/json" \
  -d '{
    "level": "ERROR",
    "timestamp": 1730715600,
    "service": "auth-service",
    "message": "Database connection timeout",
    "error_type": "db_timeout",
    "request_id": "req-101"
  }' | jq '.'

echo ""
echo "─────────────────────────────────────────────────────────────"
echo ""

# Test 2: WARN log with different format
echo -e "${YELLOW}Test 2: WARN log (lowercase)${NC}"
echo "Request:"
cat << 'EOF' | jq '.'
{
  "level": "warn",
  "timestamp": 1730715610,
  "service": "order-service",
  "message": "API response time exceeding threshold",
  "request_id": "req-102"
}
EOF

echo ""
echo "Response:"
curl -s -X POST http://$FISSION_ROUTER/transform-logs \
  -H "Content-Type: application/json" \
  -d '{
    "level": "warn",
    "timestamp": 1730715610,
    "service": "order-service",
    "message": "API response time exceeding threshold",
    "request_id": "req-102"
  }' | jq '.'

echo ""
echo "─────────────────────────────────────────────────────────────"
echo ""

# Test 3: INFO log without timestamp
echo -e "${YELLOW}Test 3: INFO log (no timestamp)${NC}"
echo "Request:"
cat << 'EOF' | jq '.'
{
  "level": "info",
  "service": "payment-service",
  "message": "Payment processed successfully",
  "request_id": "req-103"
}
EOF

echo ""
echo "Response:"
curl -s -X POST http://$FISSION_ROUTER/transform-logs \
  -H "Content-Type: application/json" \
  -d '{
    "level": "info",
    "service": "payment-service",
    "message": "Payment processed successfully",
    "request_id": "req-103"
  }' | jq '.'

echo ""
echo "─────────────────────────────────────────────────────────────"
echo ""

# Test 4: Abbreviated level format
echo -e "${YELLOW}Test 4: Abbreviated level (ERR)${NC}"
echo "Request:"
cat << 'EOF' | jq '.'
{
  "level": "ERR",
  "service": "cache-service",
  "message": "Redis connection failed"
}
EOF

echo ""
echo "Response:"
curl -s -X POST http://$FISSION_ROUTER/transform-logs \
  -H "Content-Type: application/json" \
  -d '{
    "level": "ERR",
    "service": "cache-service",
    "message": "Redis connection failed"
  }' | jq '.'

echo ""
echo "╔════════════════════════════════════════════════════════╗"
echo "║              Single Log Tests Complete!                ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "All tests passed!"