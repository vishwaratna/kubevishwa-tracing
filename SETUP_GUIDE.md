# KubeVishwa Setup Guide

This guide provides detailed step-by-step instructions for setting up the complete Kubernetes tracing solution from scratch.

## Prerequisites Verification

### 1. Check Docker Desktop and Kubernetes

```bash
# Verify Docker is running
docker version

# Verify Kubernetes is enabled and running
kubectl cluster-info

# Check if you can create resources
kubectl auth can-i create pods
```

### 2. Verify Go Installation (for development)

```bash
# Check Go version (should be 1.18+)
go version

# Verify Go modules work
go mod help
```

## Step-by-Step Setup

### Phase 1: Infrastructure Setup

#### 1.1 Create Namespace
```bash
# Create the observability namespace
kubectl create namespace observability

# Verify namespace creation
kubectl get namespaces
```

#### 1.2 Deploy Jaeger
```bash
# Deploy Jaeger all-in-one instance
kubectl apply -f k8s/jaeger-simple.yaml

# Wait for Jaeger to be ready
kubectl wait --for=condition=available --timeout=300s deployment/jaeger -n observability

# Verify Jaeger is running
kubectl get pods -n observability -l app=jaeger
```

#### 1.3 Deploy OpenTelemetry Collector
```bash
# Deploy the collector
kubectl apply -f k8s/jaeger-instance.yaml

# Wait for collector to be ready
kubectl wait --for=condition=available --timeout=300s deployment/otel-collector -n observability

# Verify collector is running
kubectl get pods -n observability -l app=otel-collector
```

#### 1.4 Verify Infrastructure
```bash
# Check all pods in observability namespace
kubectl get pods -n observability

# Check services
kubectl get services -n observability

# Test Jaeger UI accessibility
curl -s http://localhost:30686/api/services
```

### Phase 2: Custom Resource Definition

#### 2.1 Deploy TracingConfig CRD
```bash
# Apply the CRD and RBAC
kubectl apply -f k8s/tracing-crd.yaml

# Verify CRD is installed
kubectl get crd tracingconfigs.observability.kubevishwa.io

# Check RBAC
kubectl get clusterrole tracing-controller
kubectl get serviceaccount tracing-controller -n observability
```

### Phase 3: Application Deployment

#### 3.1 Build Go REST API
```bash
# Build the application binary
go build -o kubevishwa-api .

# Build Docker image
docker build -t kubevishwa-api:latest .

# Verify image was built
docker images | grep kubevishwa-api
```

#### 3.2 Build Tracing Controller
```bash
# Navigate to controller directory
cd controller

# Build the controller binary
go build -o tracing-controller .

# Build Docker image
docker build -t tracing-controller:latest .

# Verify image was built
docker images | grep tracing-controller

# Return to root directory
cd ..
```

#### 3.3 Deploy Tracing Controller
```bash
# Deploy the controller
kubectl apply -f k8s/tracing-controller-deployment.yaml

# Wait for controller to be ready
kubectl wait --for=condition=available --timeout=300s deployment/tracing-controller -n observability

# Check controller logs
kubectl logs -n observability deployment/tracing-controller --tail=20
```

#### 3.4 Deploy Go REST API
```bash
# Deploy the API and service
kubectl apply -f k8s/kubevishwa-api-deployment.yaml

# Wait for API to be ready
kubectl wait --for=condition=available --timeout=300s deployment/kubevishwa-api

# Check API pods
kubectl get pods -l app=kubevishwa-api
```

### Phase 4: Configuration and Testing

#### 4.1 Apply Tracing Configuration
```bash
# Apply the sample tracing configuration
kubectl apply -f k8s/sample-tracing-config.yaml

# Verify TracingConfig was created
kubectl get tracingconfigs

# Check if controller created ConfigMap
kubectl get configmap kubevishwa-api-tracing-tracing-config

# Verify ConfigMap contents
kubectl get configmap kubevishwa-api-tracing-tracing-config -o yaml
```

#### 4.2 Test API Connectivity
```bash
# Test health endpoint
curl http://localhost:30080/health

# Test users endpoint
curl http://localhost:30080/users

# Test specific user
curl http://localhost:30080/user?id=1

# Test products endpoint
curl http://localhost:30080/products

# Test order creation
curl -X POST http://localhost:30080/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id": 1, "product_id": 1, "quantity": 2}'
```

#### 4.3 Generate Test Traffic
```bash
# Generate multiple requests to create traces
for i in {1..10}; do
  curl -s http://localhost:30080/users > /dev/null
  curl -s http://localhost:30080/products > /dev/null
  curl -s http://localhost:30080/user?id=$((i % 3 + 1)) > /dev/null
  sleep 1
done
```

#### 4.4 Verify Traces in Jaeger
```bash
# Open Jaeger UI in browser
open http://localhost:30686

# Or test API endpoint
curl "http://localhost:30686/api/traces?service=kubevishwa-api&limit=10"
```

### Phase 5: Advanced Configuration

#### 5.1 Update Sampling Rate
```bash
# Edit the TracingConfig to change sampling rate
kubectl patch tracingconfig kubevishwa-api-tracing --type='merge' -p='{"spec":{"samplingRate":0.5}}'

# Verify the change was applied
kubectl get configmap kubevishwa-api-tracing-tracing-config -o jsonpath='{.data.OTEL_TRACES_SAMPLER_ARG}'
```

#### 5.2 Add Resource Attributes
```bash
# Update TracingConfig with additional attributes
kubectl patch tracingconfig kubevishwa-api-tracing --type='merge' -p='{
  "spec": {
    "resourceAttributes": {
      "environment": "production",
      "version": "2.0.0",
      "team": "platform"
    }
  }
}'

# Verify the changes
kubectl get configmap kubevishwa-api-tracing-tracing-config -o yaml
```

## Verification Checklist

### Infrastructure
- [ ] Jaeger pod is running in observability namespace
- [ ] OpenTelemetry Collector pod is running in observability namespace
- [ ] Jaeger UI is accessible at http://localhost:30686
- [ ] All services have correct ClusterIP addresses

### CRD and Controller
- [ ] TracingConfig CRD is installed
- [ ] Tracing controller pod is running
- [ ] Controller logs show successful reconciliation
- [ ] RBAC permissions are correctly configured

### Application
- [ ] kubevishwa-api pods are running
- [ ] API is accessible at http://localhost:30080
- [ ] Health endpoint returns 200 OK
- [ ] All API endpoints return expected responses

### Tracing
- [ ] TracingConfig resource exists and is valid
- [ ] ConfigMap with tracing configuration was created
- [ ] API deployment was updated with ConfigMap reference
- [ ] Traces appear in Jaeger UI after making requests
- [ ] Trace data includes correct service name and attributes

### End-to-End
- [ ] HTTP requests to API generate traces
- [ ] Traces are visible in Jaeger UI within 30 seconds
- [ ] Trace details show correct HTTP method, URL, and status
- [ ] Sampling rate changes are reflected in trace volume
- [ ] Resource attributes appear in trace metadata

## Troubleshooting

### Common Issues

#### 1. Pods Not Starting
```bash
# Check pod status and events
kubectl describe pod <pod-name> -n <namespace>

# Check resource constraints
kubectl top pods -n observability
kubectl top pods
```

#### 2. Controller Not Working
```bash
# Check controller logs
kubectl logs -n observability deployment/tracing-controller

# Verify RBAC permissions
kubectl auth can-i get tracingconfigs --as=system:serviceaccount:observability:tracing-controller
```

#### 3. No Traces in Jaeger
```bash
# Check OpenTelemetry Collector logs
kubectl logs -n observability deployment/otel-collector

# Verify API can reach collector
kubectl exec -it deployment/kubevishwa-api -- nslookup otel-collector.observability.svc.cluster.local

# Check API logs for tracing errors
kubectl logs deployment/kubevishwa-api
```

#### 4. Configuration Not Applied
```bash
# Check if ConfigMap exists
kubectl get configmap kubevishwa-api-tracing-tracing-config

# Verify deployment was updated
kubectl describe deployment kubevishwa-api | grep -A 10 "Environment"

# Force deployment restart
kubectl rollout restart deployment/kubevishwa-api
```

## Next Steps

After successful setup, consider:

1. **Production Hardening**: Configure TLS, authentication, and resource limits
2. **Monitoring**: Add metrics and alerting for the tracing infrastructure
3. **Scaling**: Deploy multiple replicas and configure load balancing
4. **Integration**: Connect with existing monitoring and logging systems
5. **Custom Instrumentation**: Add application-specific spans and metrics

For more advanced configurations and production deployment patterns, refer to the main README.md file.
