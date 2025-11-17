# API Gateway Deployment Quickstart

This guide provides quick deployment instructions for the API Gateway.

## Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- Docker (for local testing)
- Helm 3.0+ (optional)

## Quick Deploy to Kubernetes

### 1. Create Namespace

```bash
kubectl create namespace api-gateway
```

### 2. Install Redis (Required Dependency)

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install redis bitnami/redis \
  --namespace api-gateway \
  --set auth.enabled=false \
  --set replica.replicaCount=2
```

### 3. Create Secrets

Generate JWT keys:

```bash
# Generate RSA key pair
ssh-keygen -t rsa -b 4096 -m PEM -f jwt-signing-key.pem -N ""
openssl rsa -in jwt-signing-key.pem -pubout -outform PEM -out jwt-public-key.pem

# Create Kubernetes secret
kubectl create secret generic api-gateway-secrets \
  --from-file=jwt-public-key.pem=jwt-public-key.pem \
  --namespace=api-gateway
```

Generate TLS certificates (for testing):

```bash
# Self-signed certificate
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout tls.key -out tls.crt \
  -subj "/CN=api.example.com/O=MyOrg"

# Create TLS secret
kubectl create secret tls api-gateway-tls \
  --cert=tls.crt \
  --key=tls.key \
  --namespace=api-gateway
```

### 4. Deploy API Gateway

Using kubectl:

```bash
cd deployments/kubernetes

# Deploy all resources
kubectl apply -f rbac.yaml
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f hpa.yaml
kubectl apply -f ingress.yaml
```

Or using Kustomize (recommended):

```bash
# For development
kubectl apply -k deployments/kubernetes/overlays/dev

# For staging
kubectl apply -k deployments/kubernetes/overlays/staging

# For production
kubectl apply -k deployments/kubernetes/overlays/production
```

### 5. Verify Deployment

```bash
# Check pod status
kubectl get pods -n api-gateway

# Check service
kubectl get svc -n api-gateway

# Check logs
kubectl logs -f deployment/api-gateway -n api-gateway

# Test health endpoint
kubectl port-forward svc/api-gateway 8080:80 -n api-gateway
curl http://localhost:8080/_health/live
curl http://localhost:8080/_health/ready
```

## Quick Deploy with Docker

### 1. Build Image

```bash
docker build -t api-gateway:latest .
```

### 2. Run Container

```bash
docker run -d \
  --name api-gateway \
  -p 8080:8080 \
  -p 8443:8443 \
  -p 9090:9090 \
  -e GATEWAY_LOG_LEVEL=info \
  -e GATEWAY_REDIS_ADDR=redis:6379 \
  api-gateway:latest
```

### 3. Test

```bash
curl http://localhost:8080/_health/live
curl http://localhost:8080/_health/ready
curl http://localhost:9090/metrics
```

## Configuration Quickstart

### Environment Variables

```bash
# Server
GATEWAY_HTTP_PORT=8080
GATEWAY_HTTPS_PORT=8443

# Logging
GATEWAY_LOG_LEVEL=info  # debug, info, warn, error

# Redis
GATEWAY_REDIS_ADDR=redis:6379

# Auth
GATEWAY_JWT_PUBLIC_KEY_PATH=/app/secrets/jwt-public-key.pem
```

### ConfigMap Updates

```bash
# Edit config
kubectl edit configmap api-gateway-config -n api-gateway

# Restart to apply
kubectl rollout restart deployment/api-gateway -n api-gateway
```

## Monitoring Quickstart

### Install Prometheus & Grafana

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace
```

### Apply Monitoring Resources

```bash
cd deployments/monitoring
kubectl apply -f servicemonitor.yaml
kubectl apply -f prometheus-rules.yaml
```

### Import Grafana Dashboard

1. Access Grafana (default: http://prometheus-grafana)
2. Navigate to Dashboards → Import
3. Upload `deployments/monitoring/grafana-dashboard.json`

## Troubleshooting

### Pods Not Starting

```bash
kubectl describe pod <pod-name> -n api-gateway
kubectl logs <pod-name> -n api-gateway
```

### Check Events

```bash
kubectl get events -n api-gateway --sort-by='.lastTimestamp'
```

### Common Issues

1. **Image Pull Error**: Check image name and registry credentials
2. **CrashLoopBackOff**: Check logs for application errors
3. **ConfigMap/Secret Not Found**: Ensure they exist in the namespace

## Scaling

### Manual Scaling

```bash
kubectl scale deployment api-gateway --replicas=10 -n api-gateway
```

### Auto-scaling (HPA)

HPA is automatically configured. Adjust if needed:

```bash
kubectl edit hpa api-gateway -n api-gateway
```

## Cleanup

```bash
# Delete all resources
kubectl delete -k deployments/kubernetes/overlays/dev

# Or delete namespace (deletes everything)
kubectl delete namespace api-gateway
```

## Next Steps

- Review [Full Deployment Guide](../docs/DEPLOYMENT.md)
- Review [Operational Runbooks](../docs/RUNBOOKS.md)
- Configure monitoring and alerting
- Set up CI/CD pipeline
- Customize for your environment

## Support

- GitHub Issues: https://github.com/maltehedderich/api-gateway-go/issues
- Documentation: See `docs/` directory
- Design Spec: See `API_GATEWAY_DESIGN_SPEC.md`
