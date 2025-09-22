# KubeVishwa Troubleshooting Guide

This guide helps diagnose and resolve common issues with the KubeVishwa tracing solution.

## Quick Diagnostics

### System Health Check
```bash
# Check all components status
kubectl get pods -n observability
kubectl get pods -l app=kubevishwa-api
kubectl get tracingconfigs
kubectl get configmaps | grep tracing

# Test external access
curl -s http://localhost:30080/health
curl -s http://localhost:30686/api/services
```

## Common Issues and Solutions

### 1. Pods Not Starting

#### Symptoms
- Pods stuck in `Pending`, `CrashLoopBackOff`, or `ImagePullBackOff` state
- `kubectl get pods` shows non-running status

#### Diagnosis
```bash
# Check pod details
kubectl describe pod <pod-name> -n <namespace>

# Check events
kubectl get events --sort-by=.metadata.creationTimestamp

# Check resource usage
kubectl top nodes
kubectl top pods -A
```

#### Solutions

**ImagePullBackOff:**
```bash
# Verify images exist locally
docker images | grep -E "(kubevishwa-api|tracing-controller)"

# Rebuild images if missing
docker build -t kubevishwa-api:latest .
cd controller && docker build -t tracing-controller:latest .
```

**Resource Constraints:**
```bash
# Check node resources
kubectl describe nodes

# Reduce resource requests in deployments
kubectl patch deployment kubevishwa-api --type='merge' -p='{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "kubevishwa-api",
          "resources": {
            "requests": {"cpu": "50m", "memory": "64Mi"}
          }
        }]
      }
    }
  }
}'
```

### 2. Tracing Controller Issues

#### Symptoms
- TracingConfig resources not being processed
- ConfigMaps not created or updated
- Controller logs show errors

#### Diagnosis
```bash
# Check controller logs
kubectl logs -n observability deployment/tracing-controller --tail=50

# Check RBAC permissions
kubectl auth can-i get tracingconfigs --as=system:serviceaccount:observability:tracing-controller
kubectl auth can-i create configmaps --as=system:serviceaccount:observability:tracing-controller
kubectl auth can-i update deployments --as=system:serviceaccount:observability:tracing-controller
```

#### Solutions

**RBAC Issues:**
```bash
# Reapply RBAC configuration
kubectl apply -f k8s/tracing-crd.yaml

# Verify service account exists
kubectl get serviceaccount tracing-controller -n observability
```

**Controller Restart:**
```bash
# Restart controller
kubectl rollout restart deployment/tracing-controller -n observability

# Wait for restart
kubectl rollout status deployment/tracing-controller -n observability
```

### 3. No Traces in Jaeger

#### Symptoms
- Jaeger UI shows no traces for kubevishwa-api service
- API requests don't generate visible traces

#### Diagnosis
```bash
# Check if traces are being sent
kubectl logs deployment/kubevishwa-api | grep -i "trace\|otel\|export"

# Check OpenTelemetry Collector
kubectl logs -n observability deployment/otel-collector

# Verify collector connectivity
kubectl exec -it deployment/kubevishwa-api -- nslookup otel-collector.observability.svc.cluster.local

# Check Jaeger backend
kubectl logs -n observability deployment/jaeger
```

#### Solutions

**Network Connectivity:**
```bash
# Test collector endpoint from API pod
kubectl exec -it deployment/kubevishwa-api -- wget -qO- --timeout=5 http://otel-collector.observability.svc.cluster.local:4318/v1/traces

# Check service endpoints
kubectl get endpoints -n observability otel-collector
```

**Configuration Issues:**
```bash
# Verify tracing environment variables
kubectl exec deployment/kubevishwa-api -- env | grep OTEL

# Check ConfigMap content
kubectl get configmap kubevishwa-api-tracing-tracing-config -o yaml

# Force deployment update
kubectl rollout restart deployment/kubevishwa-api
```

**Sampling Rate:**
```bash
# Check if sampling rate is too low
kubectl get tracingconfig kubevishwa-api-tracing -o jsonpath='{.spec.samplingRate}'

# Set to 100% for testing
kubectl patch tracingconfig kubevishwa-api-tracing --type='merge' -p='{"spec":{"samplingRate":1.0}}'
```

### 4. API Connectivity Issues

#### Symptoms
- Cannot access API at http://localhost:30080
- Connection refused or timeout errors

#### Diagnosis
```bash
# Check service configuration
kubectl get service kubevishwa-api -o yaml

# Check NodePort allocation
kubectl get services --all-namespaces | grep NodePort

# Verify pods are ready
kubectl get pods -l app=kubevishwa-api -o wide
```

#### Solutions

**Service Issues:**
```bash
# Check if service endpoints exist
kubectl get endpoints kubevishwa-api

# Recreate service if needed
kubectl delete service kubevishwa-api
kubectl apply -f k8s/kubevishwa-api-deployment.yaml
```

**Port Conflicts:**
```bash
# Check if port 30080 is in use
netstat -an | grep 30080

# Use different NodePort if needed
kubectl patch service kubevishwa-api --type='merge' -p='{"spec":{"ports":[{"nodePort":30081,"port":8080,"targetPort":8080}]}}'
```

### 5. Performance Issues

#### Symptoms
- High CPU/memory usage
- Slow response times
- Trace export failures

#### Diagnosis
```bash
# Check resource usage
kubectl top pods -A
kubectl top nodes

# Check trace export metrics
kubectl logs deployment/kubevishwa-api | grep -i "export\|batch\|timeout"

# Monitor collector performance
kubectl logs -n observability deployment/otel-collector | grep -i "error\|timeout\|drop"
```

#### Solutions

**Resource Optimization:**
```bash
# Increase resource limits
kubectl patch deployment kubevishwa-api --type='merge' -p='{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "kubevishwa-api",
          "resources": {
            "limits": {"cpu": "500m", "memory": "512Mi"},
            "requests": {"cpu": "200m", "memory": "256Mi"}
          }
        }]
      }
    }
  }
}'

# Optimize batch configuration
kubectl patch tracingconfig kubevishwa-api-tracing --type='merge' -p='{
  "spec": {
    "batchConfig": {
      "maxExportBatchSize": 256,
      "scheduleDelay": "10s"
    }
  }
}'
```

## Advanced Debugging

### Enable Debug Logging

**API Debug Logging:**
```bash
# Add debug environment variable
kubectl patch deployment kubevishwa-api --type='merge' -p='{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "kubevishwa-api",
          "env": [{"name": "OTEL_LOG_LEVEL", "value": "debug"}]
        }]
      }
    }
  }
}'
```

**Controller Debug Logging:**
```bash
# Check controller with verbose logging
kubectl logs -n observability deployment/tracing-controller -f
```

### Network Debugging

**Test Internal Connectivity:**
```bash
# Create debug pod
kubectl run debug --image=nicolaka/netshoot -it --rm

# From debug pod, test connectivity
nslookup otel-collector.observability.svc.cluster.local
curl -v http://otel-collector.observability.svc.cluster.local:4318/v1/traces
```

**Check DNS Resolution:**
```bash
# Test DNS from API pod
kubectl exec deployment/kubevishwa-api -- nslookup kubernetes.default.svc.cluster.local
kubectl exec deployment/kubevishwa-api -- nslookup otel-collector.observability.svc.cluster.local
```

### Trace Validation

**Manual Trace Testing:**
```bash
# Send test trace to collector
kubectl run trace-test --image=curlimages/curl -it --rm -- \
  curl -X POST http://otel-collector.observability.svc.cluster.local:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-service"}}]},"instrumentationLibrarySpans":[{"spans":[{"traceId":"12345678901234567890123456789012","spanId":"1234567890123456","name":"test-span","startTimeUnixNano":"1609459200000000000","endTimeUnixNano":"1609459201000000000"}]}]}]}'
```

## Recovery Procedures

### Complete Reset
```bash
# Delete all resources
kubectl delete -f k8s/kubevishwa-api-deployment.yaml
kubectl delete -f k8s/tracing-controller-deployment.yaml
kubectl delete -f k8s/sample-tracing-config.yaml
kubectl delete -f k8s/tracing-crd.yaml
kubectl delete -f k8s/jaeger-instance.yaml
kubectl delete -f k8s/jaeger-simple.yaml
kubectl delete namespace observability

# Wait for cleanup
kubectl get pods -A | grep -E "(jaeger|otel|tracing|kubevishwa)"

# Redeploy from scratch
kubectl create namespace observability
kubectl apply -f k8s/jaeger-simple.yaml
kubectl apply -f k8s/jaeger-instance.yaml
kubectl apply -f k8s/tracing-crd.yaml
kubectl apply -f k8s/tracing-controller-deployment.yaml
kubectl apply -f k8s/kubevishwa-api-deployment.yaml
kubectl apply -f k8s/sample-tracing-config.yaml
```

### Partial Recovery
```bash
# Restart just the tracing components
kubectl rollout restart deployment/tracing-controller -n observability
kubectl rollout restart deployment/kubevishwa-api

# Clear and recreate tracing configuration
kubectl delete tracingconfig kubevishwa-api-tracing
kubectl delete configmap kubevishwa-api-tracing-tracing-config
kubectl apply -f k8s/sample-tracing-config.yaml
```

## Getting Help

### Collect Diagnostic Information
```bash
# Create diagnostic bundle
mkdir -p debug-info
kubectl get pods -A -o yaml > debug-info/pods.yaml
kubectl get services -A -o yaml > debug-info/services.yaml
kubectl get tracingconfigs -o yaml > debug-info/tracingconfigs.yaml
kubectl get configmaps -o yaml > debug-info/configmaps.yaml
kubectl logs -n observability deployment/tracing-controller > debug-info/controller.log
kubectl logs deployment/kubevishwa-api > debug-info/api.log
kubectl logs -n observability deployment/otel-collector > debug-info/collector.log
kubectl logs -n observability deployment/jaeger > debug-info/jaeger.log

# Create archive
tar -czf kubevishwa-debug.tar.gz debug-info/
```

### Useful Commands for Support
```bash
# System information
kubectl version
docker version
uname -a

# Cluster information
kubectl cluster-info
kubectl get nodes -o wide
kubectl get namespaces

# Resource usage
kubectl top nodes
kubectl top pods -A
```

Remember to check the main README.md for additional configuration options and the SETUP_GUIDE.md for step-by-step setup instructions.
