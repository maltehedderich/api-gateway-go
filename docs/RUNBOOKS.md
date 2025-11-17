# API Gateway Operational Runbooks

## Table of Contents

1. [High Error Rate](#high-error-rate)
2. [High Latency](#high-latency)
3. [Service Down](#service-down)
4. [Redis Connectivity Issues](#redis-connectivity-issues)
5. [High Authorization Failure Rate](#high-authorization-failure-rate)
6. [Rate Limiter Issues](#rate-limiter-issues)
7. [Backend Service Errors](#backend-service-errors)
8. [High CPU/Memory Usage](#high-cpumemory-usage)
9. [Certificate Expiration](#certificate-expiration)
10. [Pod Restart Loop](#pod-restart-loop)

---

## High Error Rate

### Alert: APIGatewayHighErrorRate

**Severity:** Critical
**Threshold:** 5xx error rate > 5%
**Duration:** 5 minutes

### Symptoms

- Increased 5xx errors in metrics/logs
- Users reporting service unavailability
- Backend services showing errors

### Investigation

```bash
# Check error rate by status code
kubectl port-forward svc/api-gateway 9090:9090 -n api-gateway
curl -s http://localhost:9090/metrics | grep gateway_http_requests_total | grep status_code

# Check recent logs for errors
kubectl logs -l app=api-gateway --tail=100 -n api-gateway | grep -i error

# Check pod status
kubectl get pods -l app=api-gateway -n api-gateway

# Check backend service health
kubectl get pods --all-namespaces
```

### Common Causes

1. **Backend Service Down**
   - Check backend service pods: `kubectl get pods -n <backend-namespace>`
   - Check backend logs: `kubectl logs <backend-pod> -n <backend-namespace>`
   - Action: Investigate and fix backend service

2. **Configuration Error**
   - Check recent config changes: `kubectl rollout history deployment/api-gateway -n api-gateway`
   - Validate config: `kubectl exec -it <pod> -n api-gateway -- /app/gateway -validate-config`
   - Action: Rollback if config change caused issue

3. **Resource Exhaustion**
   - Check resource usage: `kubectl top pods -n api-gateway`
   - Check limits: `kubectl describe pod <pod> -n api-gateway | grep -A5 Limits`
   - Action: Scale up or increase resource limits

4. **Dependency Failure (Redis)**
   - Check Redis: `kubectl get pods -l app=redis -n api-gateway`
   - Action: Restart Redis or check Redis connectivity

### Resolution Steps

1. **Identify Error Source**
   ```bash
   # Group errors by route
   kubectl logs -l app=api-gateway -n api-gateway | grep "status_code=5" | \
     jq -r '.path' | sort | uniq -c | sort -rn

   # Check backend errors
   curl -s http://localhost:9090/metrics | grep gateway_backend_requests_total
   ```

2. **Immediate Mitigation**
   ```bash
   # If specific backend failing, remove route temporarily
   kubectl edit configmap api-gateway-config -n api-gateway
   # Comment out failing route
   kubectl rollout restart deployment/api-gateway -n api-gateway

   # Or scale up if resource issue
   kubectl scale deployment api-gateway --replicas=10 -n api-gateway
   ```

3. **Fix Root Cause**
   - Fix backend service issues
   - Rollback problematic configuration
   - Increase resources if needed

4. **Verify Resolution**
   ```bash
   # Watch error rate decrease
   kubectl logs -f -l app=api-gateway -n api-gateway | grep "status_code=5"

   # Check metrics
   curl -s http://localhost:9090/metrics | grep gateway_http_requests_total
   ```

### Post-Incident

- Document root cause
- Update monitoring if needed
- Review and update capacity planning
- Consider implementing circuit breakers

---

## High Latency

### Alert: APIGatewayHighLatency / APIGatewayVeryHighLatency

**Severity:** Warning / Critical
**Threshold:** p95 > 500ms (warning), > 1s (critical)
**Duration:** 10 minutes (warning), 5 minutes (critical)

### Symptoms

- Slow response times
- Timeouts on client side
- High request duration in metrics

### Investigation

```bash
# Check latency metrics
curl -s http://localhost:9090/metrics | grep gateway_http_request_duration_seconds

# Check backend latency
curl -s http://localhost:9090/metrics | grep gateway_backend_duration_seconds

# Check resource usage
kubectl top pods -n api-gateway

# Check for CPU throttling
kubectl describe pod <pod> -n api-gateway | grep -i throttl
```

### Common Causes

1. **Backend Service Slow**
   ```bash
   # Check backend latency by service
   curl -s http://localhost:9090/metrics | \
     grep gateway_backend_duration_seconds_bucket | grep backend_service
   ```
   - Action: Investigate slow backend services

2. **Redis Latency**
   ```bash
   # Check rate limiter latency
   kubectl logs -l app=api-gateway -n api-gateway | \
     grep -i "rate.*duration"
   ```
   - Action: Check Redis performance, consider scaling Redis

3. **Resource Contention**
   ```bash
   # Check CPU and memory
   kubectl top pods -n api-gateway

   # Check if pods are being throttled
   kubectl describe pod <pod> -n api-gateway | grep -A10 "Resource Limits"
   ```
   - Action: Increase CPU/memory limits or scale horizontally

4. **High Request Volume**
   ```bash
   # Check request rate
   curl -s http://localhost:9090/metrics | \
     grep gateway_http_requests_total | tail -5
   ```
   - Action: Scale up replicas

### Resolution Steps

1. **Identify Latency Source**
   ```bash
   # Compare gateway overhead vs backend time
   # Gateway overhead = total duration - backend duration
   kubectl logs -l app=api-gateway -n api-gateway --tail=100 | \
     grep duration | jq '.duration_ms, .backend_duration_ms'
   ```

2. **Quick Mitigation**
   ```bash
   # Scale up if high load
   kubectl scale deployment api-gateway --replicas=15 -n api-gateway

   # Increase CPU limits if throttled
   kubectl edit deployment api-gateway -n api-gateway
   # Increase CPU limit to 4000m or higher

   # If Redis slow, scale Redis
   kubectl scale statefulset redis --replicas=3 -n api-gateway
   ```

3. **Fix Root Cause**
   - Optimize slow backend endpoints
   - Add caching for frequently accessed data
   - Optimize database queries in backend services
   - Increase Redis resources if rate limiting is slow

4. **Verify Resolution**
   ```bash
   # Monitor latency improvement
   watch -n 5 'curl -s http://localhost:9090/metrics | \
     grep gateway_http_request_duration_seconds_sum'
   ```

### Post-Incident

- Performance testing to identify bottlenecks
- Review and optimize authorization caching
- Consider CDN for static content
- Update capacity planning

---

## Service Down

### Alert: APIGatewayDown

**Severity:** Critical
**Threshold:** up metric == 0
**Duration:** 2 minutes

### Symptoms

- All API requests failing
- Health checks failing
- No metrics being reported
- Pods not running

### Investigation

```bash
# Check pod status
kubectl get pods -l app=api-gateway -n api-gateway

# Describe pods for events
kubectl describe pods -l app=api-gateway -n api-gateway

# Check recent events
kubectl get events -n api-gateway --sort-by='.lastTimestamp' | tail -20

# Check logs
kubectl logs -l app=api-gateway --tail=100 -n api-gateway

# Check deployments
kubectl get deployment api-gateway -n api-gateway
```

### Common Causes

1. **Image Pull Failure**
   ```bash
   kubectl describe pod <pod> -n api-gateway | grep -i "pull"
   ```
   - Action: Verify image exists and registry credentials are correct

2. **Configuration Error**
   ```bash
   kubectl logs <pod> -n api-gateway | grep -i "error\|fatal"
   ```
   - Action: Check ConfigMap and Secrets, rollback if needed

3. **Resource Limits**
   ```bash
   kubectl describe pod <pod> -n api-gateway | grep -i "insufficient"
   ```
   - Action: Increase node capacity or adjust resource requests

4. **Failed Health Checks**
   ```bash
   kubectl describe pod <pod> -n api-gateway | grep -i "liveness\|readiness"
   ```
   - Action: Check health endpoint, adjust probe settings

### Resolution Steps

1. **Check Pod Status**
   ```bash
   kubectl get pods -l app=api-gateway -n api-gateway -o wide

   # If CrashLoopBackOff or Error:
   kubectl logs <pod> -n api-gateway --previous
   ```

2. **Immediate Recovery**
   ```bash
   # Delete failed pods (will be recreated)
   kubectl delete pod <pod> -n api-gateway

   # Rollback if recent deployment caused issue
   kubectl rollout undo deployment/api-gateway -n api-gateway

   # Force restart
   kubectl rollout restart deployment/api-gateway -n api-gateway
   ```

3. **Fix Root Cause**
   - Fix configuration errors
   - Update image if corrupted
   - Increase resource limits if needed
   - Fix health check endpoints

4. **Verify Service Recovery**
   ```bash
   # Check pod status
   kubectl get pods -l app=api-gateway -n api-gateway

   # Test health endpoints
   kubectl port-forward svc/api-gateway 8080:80 -n api-gateway
   curl http://localhost:8080/_health/live
   curl http://localhost:8080/_health/ready

   # Check metrics
   curl http://localhost:9090/metrics
   ```

### Post-Incident

- Review deployment process
- Add pre-deployment validation
- Update health check configuration
- Document incident for post-mortem

---

## Redis Connectivity Issues

### Alert: APIGatewayRedisConnectivityIssues

**Severity:** Critical
**Threshold:** Redis errors > 5/second
**Duration:** 5 minutes

### Symptoms

- Rate limiting not working correctly
- Increased error logs about Redis
- Either all requests blocked or none blocked (depending on fail mode)

### Investigation

```bash
# Check Redis errors
curl -s http://localhost:9090/metrics | grep gateway_redis_errors_total

# Check Redis pod
kubectl get pods -l app=redis -n api-gateway

# Check Redis logs
kubectl logs -l app=redis -n api-gateway --tail=100

# Test Redis connectivity from gateway pod
kubectl exec -it <gateway-pod> -n api-gateway -- \
  nc -zv redis-master.api-gateway.svc.cluster.local 6379
```

### Common Causes

1. **Redis Pod Down**
   ```bash
   kubectl get pods -l app=redis -n api-gateway
   ```
   - Action: Restart Redis or investigate why it's down

2. **Network Issues**
   ```bash
   # Test DNS resolution
   kubectl exec -it <gateway-pod> -n api-gateway -- \
     nslookup redis-master.api-gateway.svc.cluster.local

   # Test connectivity
   kubectl exec -it <gateway-pod> -n api-gateway -- \
     telnet redis-master.api-gateway.svc.cluster.local 6379
   ```
   - Action: Check network policies, service configuration

3. **Redis Overloaded**
   ```bash
   # Check Redis metrics
   kubectl exec -it <redis-pod> -n api-gateway -- \
     redis-cli INFO stats
   ```
   - Action: Scale Redis or optimize rate limiting

4. **Connection Pool Exhausted**
   ```bash
   # Check pool configuration
   kubectl get configmap api-gateway-config -n api-gateway -o yaml | \
     grep -A5 pool_size
   ```
   - Action: Increase connection pool size

### Resolution Steps

1. **Verify Redis Status**
   ```bash
   kubectl get pods -l app=redis -n api-gateway
   kubectl logs -l app=redis --tail=50 -n api-gateway
   ```

2. **Quick Mitigation**
   ```bash
   # Restart Redis
   kubectl rollout restart statefulset/redis -n api-gateway

   # Or switch to fail-open mode (temporarily)
   kubectl edit configmap api-gateway-config -n api-gateway
   # Set failure_mode: fail_open
   kubectl rollout restart deployment/api-gateway -n api-gateway
   ```

3. **Fix Root Cause**
   - Fix Redis pod issues
   - Increase Redis resources
   - Optimize rate limiting configuration
   - Increase connection pool size

4. **Verify Resolution**
   ```bash
   # Check Redis errors decreased
   curl -s http://localhost:9090/metrics | grep gateway_redis_errors_total

   # Test rate limiting
   for i in {1..10}; do curl http://localhost:8080/api/v1/test; done
   ```

### Post-Incident

- Review Redis capacity and scaling
- Consider Redis clustering for HA
- Implement Redis monitoring alerts
- Document Redis failure modes

---

## High Authorization Failure Rate

### Alert: APIGatewayHighAuthFailureRate

**Severity:** Warning
**Threshold:** Auth failure rate > 20%
**Duration:** 10 minutes

### Symptoms

- Increased 401/403 errors
- Users unable to authenticate
- High auth failure metrics

### Investigation

```bash
# Check auth metrics
curl -s http://localhost:9090/metrics | grep gateway_auth_attempts_total

# Check auth error types
kubectl logs -l app=api-gateway -n api-gateway | \
  grep -i "auth.*error" | jq -r '.error_type' | sort | uniq -c

# Check JWT public key
kubectl get secret api-gateway-secrets -n api-gateway -o yaml
```

### Common Causes

1. **JWT Public Key Mismatch**
   - Signing key rotated but gateway still using old public key
   - Action: Update JWT public key secret

2. **Token Expiration Issues**
   - Clock skew between auth service and gateway
   - Tokens expiring too quickly
   - Action: Adjust clock skew tolerance

3. **Revocation List Issues**
   - Tokens incorrectly added to revocation list
   - Revocation list not syncing
   - Action: Check revocation list in Redis

4. **Authentication Service Issues**
   - Auth service issuing invalid tokens
   - Action: Investigate auth service logs

### Resolution Steps

1. **Identify Failure Type**
   ```bash
   # Group by error type
   kubectl logs -l app=api-gateway -n api-gateway --tail=1000 | \
     grep auth_status=failure | jq -r '.error_type' | sort | uniq -c | sort -rn
   ```

2. **Quick Mitigation**
   ```bash
   # If JWT key issue, update secret
   kubectl create secret generic api-gateway-secrets \
     --from-file=jwt-public-key.pem=./new-key.pem \
     --dry-run=client -o yaml | kubectl apply -f -
   kubectl rollout restart deployment/api-gateway -n api-gateway

   # If clock skew, increase tolerance
   kubectl edit configmap api-gateway-config -n api-gateway
   # Increase clock_skew: 60s
   kubectl rollout restart deployment/api-gateway -n api-gateway
   ```

3. **Fix Root Cause**
   - Coordinate JWT key rotation with auth service
   - Fix clock synchronization (NTP)
   - Clear incorrect revocation list entries
   - Fix auth service token generation

4. **Verify Resolution**
   ```bash
   # Monitor auth success rate
   kubectl logs -f -l app=api-gateway -n api-gateway | grep auth_status

   # Check metrics
   curl -s http://localhost:9090/metrics | grep gateway_auth_attempts_total
   ```

### Post-Incident

- Document JWT key rotation process
- Implement automated key rotation
- Add monitoring for auth service
- Review token lifetime settings

---

## Rate Limiter Issues

### Alert: APIGatewayRateLimiterErrors

**Severity:** Critical
**Threshold:** Rate limiter errors > 1/second
**Duration:** 5 minutes

### Symptoms

- Either all requests blocked or none blocked
- Errors about rate limiting in logs
- Redis connectivity issues

### Investigation

```bash
# Check rate limiter metrics
curl -s http://localhost:9090/metrics | grep gateway_ratelimit

# Check rate limiter errors
kubectl logs -l app=api-gateway -n api-gateway | grep -i "ratelimit.*error"

# Check Redis connectivity
kubectl exec -it <gateway-pod> -n api-gateway -- \
  nc -zv redis-master.api-gateway.svc.cluster.local 6379
```

### Common Causes

1. **Redis Down** - See [Redis Connectivity Issues](#redis-connectivity-issues)

2. **Rate Limit Configuration Error**
   ```bash
   kubectl get configmap api-gateway-config -n api-gateway -o yaml | \
     grep -A20 ratelimit
   ```

3. **Rate Limit State Corruption**
   ```bash
   # Check Redis keys
   kubectl exec -it <redis-pod> -n api-gateway -- \
     redis-cli KEYS "ratelimit:*" | head -10
   ```

### Resolution Steps

1. **Check Configuration**
   ```bash
   # Validate rate limit config
   kubectl exec -it <gateway-pod> -n api-gateway -- \
     /app/gateway -validate-config
   ```

2. **Quick Mitigation**
   ```bash
   # Switch to fail-open temporarily
   kubectl edit configmap api-gateway-config -n api-gateway
   # Set failure_mode: fail_open

   # Or disable rate limiting temporarily
   # Set enabled: false

   kubectl rollout restart deployment/api-gateway -n api-gateway
   ```

3. **Clear Corrupted State**
   ```bash
   # Clear all rate limit keys in Redis
   kubectl exec -it <redis-pod> -n api-gateway -- \
     redis-cli --scan --pattern "ratelimit:*" | \
     xargs redis-cli DEL
   ```

4. **Verify Resolution**
   ```bash
   # Test rate limiting
   for i in {1..100}; do
     curl -w "%{http_code}\n" http://localhost:8080/api/v1/test -o /dev/null -s
   done | sort | uniq -c

   # Should see mix of 200 and 429
   ```

### Post-Incident

- Review rate limit configuration
- Add rate limiter health checks
- Document rate limit tuning process
- Consider backup rate limiting strategy

---

## Backend Service Errors

### Alert: APIGatewayBackendErrors

**Severity:** Warning
**Threshold:** Backend error rate > 10%
**Duration:** 5 minutes

### Symptoms

- Specific routes returning 502/503/504 errors
- Backend latency increased
- Circuit breaker opening

### Investigation

```bash
# Check backend metrics by service
curl -s http://localhost:9090/metrics | \
  grep gateway_backend_requests_total | grep backend_service

# Check which backends are failing
kubectl logs -l app=api-gateway -n api-gateway | \
  grep backend_service | jq -r '.backend_service, .status_code' | \
  paste - - | sort | uniq -c | sort -rn

# Check circuit breaker state
curl -s http://localhost:9090/metrics | grep gateway_circuit_breaker_state
```

### Resolution Steps

1. **Identify Failing Backend**
   ```bash
   # Group errors by backend
   kubectl logs -l app=api-gateway -n api-gateway --tail=500 | \
     grep -i error | jq -r '.backend_service' | sort | uniq -c | sort -rn
   ```

2. **Check Backend Service**
   ```bash
   # Get backend pods
   kubectl get pods -l app=<backend-service> --all-namespaces

   # Check backend logs
   kubectl logs -l app=<backend-service> --tail=100

   # Test backend directly
   kubectl port-forward svc/<backend-service> 8080:8080
   curl http://localhost:8080/health
   ```

3. **Quick Mitigation**
   ```bash
   # Remove failing route temporarily
   kubectl edit configmap api-gateway-config -n api-gateway
   # Comment out failing route
   kubectl rollout restart deployment/api-gateway -n api-gateway

   # Or restart backend service
   kubectl rollout restart deployment/<backend-service>
   ```

4. **Verify Resolution**
   ```bash
   # Check backend error rate decreased
   curl -s http://localhost:9090/metrics | \
     grep gateway_backend_requests_total | grep <backend-service>
   ```

### Post-Incident

- Fix backend service issues
- Review timeout configurations
- Implement circuit breaker patterns
- Add backend service monitoring

---

## High CPU/Memory Usage

### Alert: APIGatewayHighCPU / APIGatewayHighMemory

**Severity:** Warning
**Threshold:** CPU > 1.5 cores, Memory > 800MB
**Duration:** 10 minutes

### Investigation

```bash
# Check resource usage
kubectl top pods -n api-gateway

# Check resource limits
kubectl describe pod <pod> -n api-gateway | grep -A10 "Limits:"

# Check for memory leaks
kubectl logs <pod> -n api-gateway | grep -i "out of memory\|oom"
```

### Resolution Steps

1. **Immediate Mitigation**
   ```bash
   # Scale horizontally
   kubectl scale deployment api-gateway --replicas=10 -n api-gateway

   # Increase resource limits
   kubectl edit deployment api-gateway -n api-gateway
   # Increase CPU/memory limits
   ```

2. **Investigate High Usage**
   ```bash
   # Check request rate
   curl -s http://localhost:9090/metrics | grep gateway_http_requests_total

   # Check goroutine count
   curl -s http://localhost:9090/metrics | grep go_goroutines

   # Profile the application (if profiling enabled)
   kubectl port-forward <pod> 6060:6060 -n api-gateway
   go tool pprof http://localhost:6060/debug/pprof/heap
   ```

3. **Verify Resolution**
   ```bash
   # Monitor resource usage
   watch kubectl top pods -n api-gateway
   ```

### Post-Incident

- Performance profiling
- Optimize code if needed
- Review capacity planning
- Update resource requests/limits

---

## Certificate Expiration

### Symptoms

- TLS errors
- Browser warnings about invalid certificates
- Certificate expiration alerts

### Investigation

```bash
# Check certificate expiration
kubectl get certificate -n api-gateway

# Describe certificate
kubectl describe certificate api-gateway-tls -n api-gateway

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager
```

### Resolution Steps

```bash
# If using cert-manager, force renewal
kubectl delete secret api-gateway-tls -n api-gateway
# cert-manager will automatically recreate

# Or manually update certificate
kubectl create secret tls api-gateway-tls \
  --cert=new-tls.crt \
  --key=new-tls.key \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart pods to pick up new certificate
kubectl rollout restart deployment/api-gateway -n api-gateway
```

### Post-Incident

- Set up certificate expiration monitoring
- Configure automated renewal
- Document certificate management process

---

## Pod Restart Loop

### Alert: APIGatewayHighRestartRate

**Severity:** Warning
**Threshold:** Restarts > 0 in 15 minutes
**Duration:** 5 minutes

### Investigation

```bash
# Check restart count
kubectl get pods -l app=api-gateway -n api-gateway

# Check pod events
kubectl describe pod <pod> -n api-gateway | grep -A20 Events

# Check logs from previous container
kubectl logs <pod> -n api-gateway --previous

# Check if OOMKilled
kubectl describe pod <pod> -n api-gateway | grep -i "oom\|killed"
```

### Resolution Steps

1. **Identify Restart Cause**
   ```bash
   # Check exit code
   kubectl describe pod <pod> -n api-gateway | grep "Exit Code"

   # Exit Code 137 = OOMKilled (increase memory)
   # Exit Code 1 = Application error (check logs)
   # Exit Code 0 = Clean shutdown (check health probes)
   ```

2. **Fix Based on Cause**
   ```bash
   # If OOMKilled, increase memory
   kubectl edit deployment api-gateway -n api-gateway
   # Increase memory limits

   # If health check failing, adjust probes
   kubectl edit deployment api-gateway -n api-gateway
   # Increase initialDelaySeconds or failureThreshold

   # If application error, check logs and fix configuration
   kubectl logs <pod> -n api-gateway --previous
   ```

3. **Verify Stability**
   ```bash
   # Watch pods
   watch kubectl get pods -l app=api-gateway -n api-gateway

   # Check restart count remains 0
   ```

### Post-Incident

- Fix application bugs if crash
- Tune resource limits appropriately
- Review health check configuration
- Add monitoring for container restarts

---

## Emergency Contacts

- **On-Call Engineer**: Check PagerDuty rotation
- **Platform Team**: platform-team@example.com
- **Backend Team**: backend-team@example.com
- **Security Team**: security@example.com

## Escalation Path

1. On-call engineer investigates
2. Escalate to platform team if infrastructure issue
3. Escalate to backend team if backend service issue
4. Page management for extended outages (> 1 hour)

## Useful Commands Reference

```bash
# Quick health check
kubectl get pods -n api-gateway && \
  kubectl logs -l app=api-gateway --tail=10 -n api-gateway && \
  curl -s http://localhost:9090/metrics | grep -E "up|gateway_http_requests_total"

# Full system check
kubectl get all -n api-gateway
kubectl top pods -n api-gateway
kubectl get events -n api-gateway --sort-by='.lastTimestamp' | tail -20

# Emergency rollback
kubectl rollout undo deployment/api-gateway -n api-gateway

# Emergency scale down/up
kubectl scale deployment api-gateway --replicas=0 -n api-gateway
kubectl scale deployment api-gateway --replicas=5 -n api-gateway
```
