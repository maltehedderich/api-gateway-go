# API Gateway Deployment Guide

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Docker Deployment](#docker-deployment)
3. [Kubernetes Deployment](#kubernetes-deployment)
4. [Configuration](#configuration)
5. [Secrets Management](#secrets-management)
6. [Monitoring Setup](#monitoring-setup)
7. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Tools

- **Docker** 20.10+
- **Kubernetes** 1.24+
- **kubectl** configured for your cluster
- **Helm** 3.0+ (optional, for dependency installation)
- **Go** 1.21+ (for building from source)

### Required Services

- **Redis** 6.0+ (for rate limiting)
- **Prometheus** (for metrics collection)
- **Grafana** (for dashboards)

---

## Docker Deployment

### Build Docker Image

```bash
# From repository root
docker build -t api-gateway:latest .

# Build with version tag
VERSION=$(git describe --tags --always --dirty)
docker build -t api-gateway:${VERSION} .

# Tag for registry
docker tag api-gateway:${VERSION} your-registry.example.com/api-gateway:${VERSION}
docker push your-registry.example.com/api-gateway:${VERSION}
```

### Run Docker Container

#### Basic Run

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

#### Run with Custom Configuration

```bash
docker run -d \
  --name api-gateway \
  -p 8080:8080 \
  -p 8443:8443 \
  -p 9090:9090 \
  -v $(pwd)/configs/config.prod.yaml:/app/configs/config.yaml:ro \
  -v $(pwd)/secrets:/app/secrets:ro \
  -e GATEWAY_LOG_LEVEL=info \
  api-gateway:latest -config /app/configs/config.yaml
```

#### Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  api-gateway:
    build: .
    ports:
      - "8080:8080"
      - "8443:8443"
      - "9090:9090"
    environment:
      - GATEWAY_LOG_LEVEL=info
      - GATEWAY_REDIS_ADDR=redis:6379
    volumes:
      - ./configs/config.prod.yaml:/app/configs/config.yaml:ro
      - ./secrets:/app/secrets:ro
    depends_on:
      - redis
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/_health/live"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes

volumes:
  redis-data:
```

Run with Docker Compose:

```bash
docker-compose up -d
```

---

## Kubernetes Deployment

### Quick Start

```bash
# Navigate to deployments directory
cd deployments/kubernetes

# Create namespace (if needed)
kubectl create namespace api-gateway

# Apply manifests in order
kubectl apply -f rbac.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f hpa.yaml
kubectl apply -f ingress.yaml
```

### Step-by-Step Deployment

#### 1. Create Namespace

```bash
kubectl create namespace api-gateway
kubectl config set-context --current --namespace=api-gateway
```

#### 2. Install Dependencies

**Install Redis:**

```bash
# Using Helm
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install redis bitnami/redis \
  --namespace api-gateway \
  --set auth.enabled=false \
  --set replica.replicaCount=2 \
  --set metrics.enabled=true
```

**Install Prometheus & Grafana:**

```bash
# Using kube-prometheus-stack
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace
```

#### 3. Create Secrets

**Generate JWT Keys:**

```bash
# Generate RSA key pair for JWT
ssh-keygen -t rsa -b 4096 -m PEM -f jwt-signing-key.pem -N ""
openssl rsa -in jwt-signing-key.pem -pubout -outform PEM -out jwt-public-key.pem

# Create secret
kubectl create secret generic api-gateway-secrets \
  --from-file=jwt-public-key.pem=jwt-public-key.pem \
  --namespace=api-gateway
```

**Generate TLS Certificates:**

For testing:

```bash
# Self-signed certificate
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout tls.key -out tls.crt \
  -subj "/CN=api.example.com/O=MyOrg"

# Create or update secret
kubectl create secret tls api-gateway-tls \
  --cert=tls.crt \
  --key=tls.key \
  --namespace=api-gateway
```

For production, use cert-manager:

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer for Let's Encrypt
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF
```

#### 4. Configure ConfigMap

Edit `configmap.yaml` to match your environment:

```bash
# Update Redis address
kubectl edit configmap api-gateway-config -n api-gateway

# Or apply updated file
kubectl apply -f configmap.yaml
```

#### 5. Deploy Application

```bash
# Apply RBAC
kubectl apply -f rbac.yaml

# Apply ConfigMap
kubectl apply -f configmap.yaml

# Apply Secrets (if not created via kubectl create)
# kubectl apply -f secrets.yaml

# Deploy application
kubectl apply -f deployment.yaml

# Create services
kubectl apply -f service.yaml

# Configure ingress
kubectl apply -f ingress.yaml

# Enable autoscaling
kubectl apply -f hpa.yaml
```

#### 6. Setup Monitoring

```bash
cd ../monitoring

# Apply ServiceMonitor (requires Prometheus Operator)
kubectl apply -f servicemonitor.yaml

# Apply Prometheus rules
kubectl apply -f prometheus-rules.yaml

# Import Grafana dashboard
# Navigate to Grafana UI and import grafana-dashboard.json
```

#### 7. Verify Deployment

```bash
# Check pod status
kubectl get pods -n api-gateway

# Check service
kubectl get svc -n api-gateway

# Check ingress
kubectl get ingress -n api-gateway

# Check logs
kubectl logs -f deployment/api-gateway -n api-gateway

# Check health
kubectl port-forward svc/api-gateway 8080:80 -n api-gateway
curl http://localhost:8080/_health/live
curl http://localhost:8080/_health/ready
```

---

## Configuration

### Environment Variables

All configuration can be overridden with environment variables using `GATEWAY_` prefix:

```bash
# Server configuration
GATEWAY_HTTP_PORT=8080
GATEWAY_HTTPS_PORT=8443
GATEWAY_METRICS_PORT=9090

# Logging
GATEWAY_LOG_LEVEL=info          # debug, info, warn, error
GATEWAY_LOG_FORMAT=json         # json, text

# Redis
GATEWAY_REDIS_ADDR=redis:6379
GATEWAY_REDIS_PASSWORD=secret
GATEWAY_REDIS_DB=0

# Auth
GATEWAY_SESSION_COOKIE_NAME=session_token
GATEWAY_JWT_PUBLIC_KEY_PATH=/app/secrets/jwt-public-key.pem
```

### Configuration Files

Configuration files are located in `configs/`:

- `config.dev.yaml` - Development environment
- `config.staging.yaml` - Staging environment
- `config.prod.yaml` - Production environment

### Kubernetes ConfigMap

Update ConfigMap for runtime configuration:

```bash
kubectl edit configmap api-gateway-config -n api-gateway
```

For configuration reload without restart (if supported):

```bash
kubectl rollout restart deployment/api-gateway -n api-gateway
```

---

## Secrets Management

### Kubernetes Secrets

#### Create Secrets

```bash
# From files
kubectl create secret generic api-gateway-secrets \
  --from-file=jwt-public-key.pem=./jwt-public-key.pem \
  --from-file=tls.crt=./tls.crt \
  --from-file=tls.key=./tls.key \
  --namespace=api-gateway

# From literals
kubectl create secret generic api-gateway-secrets \
  --from-literal=redis-password=mypassword \
  --namespace=api-gateway
```

#### Update Secrets

```bash
# Delete and recreate
kubectl delete secret api-gateway-secrets -n api-gateway
kubectl create secret generic api-gateway-secrets \
  --from-file=jwt-public-key.pem=./new-jwt-public-key.pem \
  --namespace=api-gateway

# Restart pods to pick up new secrets
kubectl rollout restart deployment/api-gateway -n api-gateway
```

### Using External Secrets Management

#### HashiCorp Vault

Install Vault Agent Injector:

```bash
helm repo add hashicorp https://helm.releases.hashicorp.com
helm install vault hashicorp/vault \
  --namespace vault \
  --create-namespace
```

Configure secrets injection (see Vault documentation).

#### AWS Secrets Manager

Use External Secrets Operator:

```bash
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets \
  external-secrets/external-secrets \
  --namespace external-secrets-system \
  --create-namespace
```

Create ExternalSecret resource:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: api-gateway-secrets
  namespace: api-gateway
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: api-gateway-secrets
  data:
  - secretKey: jwt-public-key.pem
    remoteRef:
      key: api-gateway/jwt-public-key
```

---

## Monitoring Setup

### Prometheus Configuration

ServiceMonitor is automatically configured when using Prometheus Operator.

Manual Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'api-gateway'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - api-gateway
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: api-gateway
      - source_labels: [__meta_kubernetes_pod_container_port_name]
        action: keep
        regex: metrics
```

### Grafana Dashboard

Import dashboard:

1. Open Grafana UI
2. Navigate to Dashboards → Import
3. Upload `deployments/monitoring/grafana-dashboard.json`
4. Select Prometheus datasource
5. Click Import

### Alerts

Prometheus alerts are defined in `prometheus-rules.yaml`.

To view active alerts:

```bash
# Port forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# Open http://localhost:9090/alerts
```

Configure alert receivers in AlertManager:

```yaml
# alertmanager-config.yaml
receivers:
  - name: 'slack'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
        channel: '#alerts'
        title: 'API Gateway Alert'

  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: 'YOUR_PAGERDUTY_SERVICE_KEY'

route:
  receiver: 'slack'
  group_by: ['alertname', 'severity']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  routes:
    - match:
        severity: critical
      receiver: pagerduty
      continue: true
```

---

## Troubleshooting

### Common Issues

#### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n api-gateway

# Describe pod for events
kubectl describe pod <pod-name> -n api-gateway

# Check logs
kubectl logs <pod-name> -n api-gateway

# Common causes:
# - Image pull errors (check image name and credentials)
# - ConfigMap/Secret not found (verify they exist)
# - Resource limits too low (check CPU/memory)
# - Health checks failing (check /_health/live endpoint)
```

#### Configuration Issues

```bash
# Validate configuration
kubectl exec -it <pod-name> -n api-gateway -- \
  /app/gateway -config /app/configs/config.yaml -validate-config

# Check ConfigMap
kubectl get configmap api-gateway-config -n api-gateway -o yaml

# Check Secrets
kubectl get secret api-gateway-secrets -n api-gateway
```

#### Redis Connectivity

```bash
# Test Redis connection from pod
kubectl exec -it <pod-name> -n api-gateway -- \
  wget -qO- http://localhost:9090/metrics | grep redis

# Check Redis pod
kubectl get pods -l app=redis -n api-gateway
kubectl logs -l app=redis -n api-gateway
```

#### High Latency

```bash
# Check metrics
kubectl port-forward svc/api-gateway 9090:9090 -n api-gateway
curl http://localhost:9090/metrics | grep duration

# Check resource usage
kubectl top pods -n api-gateway

# Check backend services
kubectl get pods --all-namespaces
```

#### Certificate Issues

```bash
# Check cert-manager certificates
kubectl get certificates -n api-gateway
kubectl describe certificate api-gateway-tls -n api-gateway

# Check TLS secret
kubectl get secret api-gateway-tls -n api-gateway -o yaml
```

### Debugging Commands

```bash
# Shell into pod
kubectl exec -it <pod-name> -n api-gateway -- /bin/sh

# View environment variables
kubectl exec <pod-name> -n api-gateway -- env

# Test health endpoints
kubectl exec <pod-name> -n api-gateway -- \
  wget -qO- http://localhost:8080/_health/live

# Check metrics endpoint
kubectl exec <pod-name> -n api-gateway -- \
  wget -qO- http://localhost:9090/metrics
```

### Performance Tuning

#### Adjust Resource Limits

```bash
kubectl edit deployment api-gateway -n api-gateway

# Update resources:
resources:
  requests:
    cpu: 1000m
    memory: 1Gi
  limits:
    cpu: 4000m
    memory: 2Gi
```

#### Adjust Autoscaling

```bash
kubectl edit hpa api-gateway -n api-gateway

# Update scaling parameters
minReplicas: 5
maxReplicas: 30
```

#### Optimize Connection Pooling

Update ConfigMap to adjust connection pool sizes for Redis and backend services.

---

## Rollback

### Kubernetes Rollback

```bash
# View rollout history
kubectl rollout history deployment/api-gateway -n api-gateway

# Rollback to previous version
kubectl rollout undo deployment/api-gateway -n api-gateway

# Rollback to specific revision
kubectl rollout undo deployment/api-gateway --to-revision=2 -n api-gateway

# Check rollout status
kubectl rollout status deployment/api-gateway -n api-gateway
```

### Docker Rollback

```bash
# Stop current container
docker stop api-gateway
docker rm api-gateway

# Run previous version
docker run -d \
  --name api-gateway \
  -p 8080:8080 \
  api-gateway:previous-version
```

---

## Maintenance

### Updating Configuration

```bash
# Update ConfigMap
kubectl apply -f configmap.yaml

# Restart deployment to pick up changes
kubectl rollout restart deployment/api-gateway -n api-gateway
```

### Updating Secrets

```bash
# Update secret
kubectl delete secret api-gateway-secrets -n api-gateway
kubectl create secret generic api-gateway-secrets \
  --from-file=jwt-public-key.pem=./new-key.pem \
  --namespace=api-gateway

# Restart deployment
kubectl rollout restart deployment/api-gateway -n api-gateway
```

### Scaling

```bash
# Manual scaling
kubectl scale deployment api-gateway --replicas=10 -n api-gateway

# Update HPA
kubectl edit hpa api-gateway -n api-gateway
```

### Upgrading

```bash
# Update image version in deployment.yaml
# Then apply:
kubectl apply -f deployment.yaml

# Or use kubectl set image:
kubectl set image deployment/api-gateway \
  gateway=api-gateway:v2.0.0 \
  -n api-gateway

# Watch rollout
kubectl rollout status deployment/api-gateway -n api-gateway
```

---

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [cert-manager Documentation](https://cert-manager.io/docs/)
- [API Gateway Design Specification](../API_GATEWAY_DESIGN_SPEC.md)
- [Operational Runbooks](RUNBOOKS.md)
