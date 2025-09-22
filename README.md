# KubeVishwa - Complete Kubernetes Tracing Solution

A comprehensive distributed tracing solution for Kubernetes that includes a Go REST API, custom resource definitions (CRDs), and automated tracing configuration management.

## 🏗️ Architecture Overview

This solution consists of the following components:

1. **Go REST API Application** (`kubevishwa-api`) - A sample microservice instrumented with OpenTelemetry
2. **Custom Resource Definition (CRD)** - `TracingConfig` for managing tracing policies
3. **Tracing Controller** - Kubernetes controller that watches TracingConfig resources and applies configurations
4. **Tracing Infrastructure** - Jaeger and OpenTelemetry Collector for trace collection and visualization

## 📋 Prerequisites

- Docker Desktop with Kubernetes enabled
- kubectl configured to access your cluster
- Go 1.18+ (for development)

## 🚀 Quick Start

### 1. Setup Tracing Infrastructure

```bash
# Create observability namespace
kubectl create namespace observability

# Deploy Jaeger and OpenTelemetry Collector
kubectl apply -f k8s/jaeger-simple.yaml
kubectl apply -f k8s/jaeger-instance.yaml
```

### 2. Deploy Custom Resource Definition

```bash
# Deploy the TracingConfig CRD and RBAC
kubectl apply -f k8s/tracing-crd.yaml
```

### 3. Build and Deploy Applications

```bash
# Build the Go API
docker build -t kubevishwa-api:latest .

# Build the tracing controller
cd controller
docker build -t tracing-controller:latest .
cd ..

# Deploy applications
kubectl apply -f k8s/tracing-controller-deployment.yaml
kubectl apply -f k8s/kubevishwa-api-deployment.yaml
```

### 4. Configure Tracing

```bash
# Apply tracing configuration
kubectl apply -f k8s/sample-tracing-config.yaml
```

### 5. Test the Solution

```bash
# Generate some traces
curl http://localhost:30080/users
curl http://localhost:30080/products
curl http://localhost:30080/user?id=1

# View traces in Jaeger UI
open http://localhost:30686
```

## 🔧 Components Deep Dive

### Go REST API (`kubevishwa-api`)

A sample REST API with the following endpoints:
- `GET /health` - Health check
- `GET /users` - List all users
- `GET /user?id=<id>` - Get specific user
- `GET /products` - List all products
- `POST /orders` - Create a new order

**Tracing Features:**
- Automatic HTTP request tracing
- Custom span attributes
- OpenTelemetry SDK integration
- OTLP export to collector

### TracingConfig CRD

Custom resource for managing tracing configurations:

```yaml
apiVersion: observability.kubevishwa.io/v1
kind: TracingConfig
metadata:
  name: kubevishwa-api-tracing
spec:
  targetSelector:
    matchLabels:
      app: kubevishwa-api
  samplingRate: 1.0
  endpoint: "http://otel-collector.observability.svc.cluster.local:4317"
  batchConfig:
    maxExportBatchSize: 512
    scheduleDelay: "5s"
    exportTimeout: "30s"
  resourceAttributes:
    environment: "development"
    version: "1.0.0"
```

### Tracing Controller

Kubernetes controller that:
- Watches TracingConfig resources
- Creates ConfigMaps with tracing environment variables
- Updates target deployments to inject tracing configuration
- Manages tracing lifecycle automatically

## 📊 Data Flow

1. **Configuration Phase:**
   - User creates/updates TracingConfig resource
   - Controller watches for changes
   - Controller creates/updates ConfigMap with tracing settings
   - Controller updates target deployment to use ConfigMap

2. **Request Phase:**
   - User sends HTTP request to API
   - OpenTelemetry SDK creates spans automatically
   - Spans include HTTP method, URL, status code, etc.
   - API processes request and returns response

3. **Tracing Phase:**
   - OpenTelemetry SDK exports traces via OTLP
   - OpenTelemetry Collector receives and processes traces
   - Collector forwards traces to Jaeger backend
   - Jaeger stores and indexes trace data

4. **Visualization Phase:**
   - User accesses Jaeger UI
   - Jaeger UI queries backend for traces
   - Traces are displayed with timing and dependency information

## 🛠️ Configuration Options

### Sampling Rates
- `1.0` - 100% sampling (development)
- `0.1` - 10% sampling (production)
- `0.01` - 1% sampling (high-traffic production)

### Batch Configuration
- `maxExportBatchSize` - Number of spans per batch
- `scheduleDelay` - Time between batch exports
- `exportTimeout` - Timeout for export operations

### Resource Attributes
Add custom attributes to all traces:
- `environment` - deployment environment
- `version` - application version
- `team` - owning team
- `region` - deployment region

## 🔍 Monitoring and Troubleshooting

### Check Component Status
```bash
# Check all pods
kubectl get pods -n observability
kubectl get pods -n default

# Check controller logs
kubectl logs -n observability deployment/tracing-controller

# Check API logs
kubectl logs -n default deployment/kubevishwa-api
```

### Verify Tracing Configuration
```bash
# Check TracingConfig resources
kubectl get tracingconfigs

# Check generated ConfigMaps
kubectl get configmap kubevishwa-api-tracing-tracing-config -o yaml

# Check if deployment was updated
kubectl describe deployment kubevishwa-api
```

### Test Connectivity
```bash
# Test API connectivity
curl http://localhost:30080/health

# Test Jaeger UI
curl http://localhost:30686/api/services

# Test OpenTelemetry Collector
kubectl port-forward -n observability svc/otel-collector 4317:4317
```

## 📈 Scaling and Production Considerations

### High Availability
- Deploy multiple replicas of the tracing controller
- Use Jaeger production deployment with external storage
- Configure OpenTelemetry Collector with multiple replicas

### Performance
- Adjust sampling rates based on traffic volume
- Configure appropriate batch sizes
- Monitor collector resource usage

### Security
- Use TLS for OTLP connections
- Implement proper RBAC policies
- Secure Jaeger UI access

## 🧪 Testing

### Unit Tests
```bash
# Run API tests
go test ./...

# Run controller tests
cd controller && go test ./...
```

### Integration Tests
```bash
# Test complete flow
./scripts/integration-test.sh
```

## 📚 Additional Resources

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/instrumentation/go/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Controller Runtime](https://github.com/kubernetes-sigs/controller-runtime)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.
