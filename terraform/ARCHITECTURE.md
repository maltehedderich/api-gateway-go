# Serverless API Gateway Architecture

## Overview

This document describes the serverless AWS architecture for the API Gateway implementation.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                          Internet                                │
└──────────────────────────────┬──────────────────────────────────┘
                               │ HTTPS
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│            AWS API Gateway HTTP API                              │
│  • JWT Authorization (RS256/HS256)                               │
│  • Global + Per-Route Throttling                                 │
│  • CORS Configuration                                            │
│  • CloudWatch Access Logs                                        │
│  • Custom Domain Support (Optional)                              │
└───────┬────────────┬────────────┬────────────┬───────────────────┘
        │            │            │            │
        ▼            ▼            ▼            ▼
   ┌────────┐  ┌─────────┐  ┌─────────┐  ┌──────────┐
   │ Lambda │  │ Lambda  │  │ Lambda  │  │ Lambda   │
   │  User  │  │ Order   │  │ Admin   │  │ Status   │
   │Service │  │ Service │  │ Service │  │ Service  │
   └────┬───┘  └────┬────┘  └────┬────┘  └────┬─────┘
        │           │            │            │
        │  ┌────────┴────────────┘            │
        │  │                                  │
        ▼  ▼                                  │
   ┌──────────────────────────┐              │
   │   DynamoDB Tables        │              │
   │  • users (on-demand)     │              │
   │  • orders (on-demand)    │              │
   │  • Encrypted at rest     │              │
   │  • GSI for queries       │              │
   └──────────────────────────┘              │
                                             │
   ┌──────────────────────────────────────────┘
   │
   ▼
┌─────────────────────────────────────────────────────────────────┐
│                    CloudWatch Logs                               │
│  • API Gateway Access Logs (7-day retention)                     │
│  • Lambda Function Logs (7-day retention)                        │
│  • Structured JSON logging                                       │
│  • X-Ray Tracing Integration                                     │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### 1. AWS API Gateway HTTP API

**Purpose:** Entry point for all API requests, handles HTTPS termination, JWT authorization, and request routing.

**Features:**
- **Protocol:** HTTP API (not REST API) for cost optimization (70% cheaper)
- **Authorization:** Native JWT authorizer with configurable issuer and audience
- **Throttling:** Global and per-route rate limiting (50 req/s, 100 burst by default)
- **CORS:** Configurable cross-origin resource sharing
- **Logging:** Structured JSON access logs to CloudWatch
- **Custom Domain:** Optional ACM certificate integration with Route53

**Cost:** $1.00 per million requests (after 1M free tier)

**Endpoints:**
- `/api/v1/users/*` → user-service
- `/api/v1/orders/*` → order-service
- `/api/v1/admin/*` → admin-service
- `/_health/*` → status-service

---

### 2. AWS Lambda Functions

**Purpose:** Serverless compute for backend business logic.

**Configuration:**
- **Runtime:** Go custom runtime (provided.al2023)
- **Memory:** 256 MB (configurable per function)
- **Timeout:** 10 seconds (configurable)
- **Concurrency:** Unreserved (-1)
- **Tracing:** AWS X-Ray enabled

**Functions:**

#### a. user-service
- **Routes:** `GET/POST /api/v1/users`, `GET/PUT/DELETE /api/v1/users/{id}`
- **Authorization:** JWT required
- **DynamoDB Access:** `users` table (read/write)
- **Functionality:** CRUD operations for user management

#### b. order-service
- **Routes:** `GET/POST /api/v1/orders`, `GET/PUT/DELETE /api/v1/orders/{id}`
- **Authorization:** JWT required
- **DynamoDB Access:** `orders` table (read/write)
- **Functionality:** Order creation, retrieval, update, deletion

#### c. admin-service
- **Routes:** `GET/POST /api/v1/admin`
- **Authorization:** JWT required
- **Throttling:** Stricter limits (20 req/s)
- **Functionality:** Administrative operations and statistics

#### d. status-service
- **Routes:** `GET /_health`, `GET /_health/ready`, `GET /_health/live`, `GET /api/v1/public/health`
- **Authorization:** None (public)
- **Functionality:** Health checks and system status

**Cost:** $0.20 per million requests + $0.0000166667 per GB-second (free tier: 1M requests + 400K GB-seconds)

---

### 3. Amazon DynamoDB

**Purpose:** NoSQL database for persistent data storage.

**Configuration:**
- **Billing Mode:** On-demand (pay-per-request)
- **Encryption:** AWS managed keys (free)
- **Point-in-Time Recovery:** Disabled (enable for production)
- **Deletion Protection:** Disabled (enable for production)

**Tables:**

#### a. users
- **Hash Key:** `user_id` (String)
- **Attributes:** `user_id`, `email`, `name`, `created_at`
- **GSI:** `email-index` (hash key: `email`)
- **Purpose:** User profile storage

#### b. orders
- **Hash Key:** `order_id` (String)
- **Range Key:** `created_at` (Number)
- **Attributes:** `order_id`, `created_at`, `user_id`, `product`, `quantity`, `total`, `status`
- **GSI:** `user-orders-index` (hash key: `user_id`, range key: `created_at`)
- **Purpose:** Order tracking and history

**Cost:**
- $1.25 per million write requests
- $0.25 per million read requests
- $0.25 per GB-month storage (25 GB free tier)

---

### 4. Amazon CloudWatch

**Purpose:** Logging, monitoring, and observability.

**Components:**

#### a. CloudWatch Logs
- **API Gateway Logs:** `/aws/apigateway/api-gateway-{environment}`
- **Lambda Logs:** `/aws/lambda/api-gateway-{environment}-{service}`
- **Retention:** 7 days (configurable)
- **Format:** Structured JSON

#### b. CloudWatch Metrics
- **API Gateway:** Request count, latency, 4xx/5xx errors
- **Lambda:** Invocations, duration, errors, throttles
- **DynamoDB:** Consumed capacity, throttled requests

#### c. AWS X-Ray
- **Tracing:** End-to-end request tracing
- **Service Map:** Visual representation of service dependencies
- **Latency Analysis:** Identify bottlenecks

**Cost:**
- 5 GB ingestion + 5 GB storage free
- $0.50 per GB ingested (after free tier)

---

### 5. AWS IAM

**Purpose:** Identity and access management with least-privilege principles.

**Roles:**

#### a. Lambda Execution Roles
- **Permissions:**
  - CloudWatch Logs: CreateLogGroup, CreateLogStream, PutLogEvents
  - DynamoDB: GetItem, PutItem, UpdateItem, DeleteItem, Query, Scan
  - X-Ray: PutTraceSegments, PutTelemetryRecords
- **Trust Policy:** Allow `lambda.amazonaws.com` to assume role

#### b. API Gateway Execution Role
- **Permissions:** Invoke Lambda functions
- **Trust Policy:** Allow `apigateway.amazonaws.com` to assume role

**Cost:** Free

---

### 6. AWS Certificate Manager (Optional)

**Purpose:** SSL/TLS certificate management for custom domains.

**Configuration:**
- **Domain:** Custom domain (e.g., `api.example.com`)
- **Validation:** DNS validation
- **Region:** Same as API Gateway (for Regional API)

**Cost:** Free

---

### 7. Amazon Route53 (Optional)

**Purpose:** DNS management for custom domains.

**Configuration:**
- **Record Type:** A record (alias to API Gateway domain)
- **Hosted Zone:** Required for DNS management

**Cost:** $0.50 per hosted zone per month + $0.40 per million queries

---

## Request Flow

### Public Health Check Request

```
1. Client sends GET /_health
2. API Gateway receives request
3. API Gateway logs request to CloudWatch
4. API Gateway invokes status-service Lambda (no authorization)
5. Lambda executes and returns {"status": "healthy", ...}
6. API Gateway returns response to client
7. API Gateway logs response to CloudWatch
```

### Authenticated User Request

```
1. Client sends GET /api/v1/users with Authorization header
2. API Gateway receives request
3. API Gateway validates JWT token:
   a. Extracts JWT from Authorization header
   b. Verifies signature using JWKS from jwt_issuer
   c. Validates issuer, audience, and expiration
4. If JWT is valid:
   a. API Gateway passes JWT claims to Lambda via context
   b. API Gateway invokes user-service Lambda
   c. Lambda queries DynamoDB users table
   d. Lambda returns user list
   e. API Gateway returns response to client
5. If JWT is invalid:
   a. API Gateway returns 401 Unauthorized
6. API Gateway logs request/response to CloudWatch
```

### Order Creation Request

```
1. Client sends POST /api/v1/orders with JWT and order data
2. API Gateway validates JWT (same as above)
3. API Gateway invokes order-service Lambda
4. Lambda extracts user_id from JWT claims
5. Lambda generates order_id and timestamp
6. Lambda writes order to DynamoDB orders table
7. Lambda returns order details to client via API Gateway
8. CloudWatch logs capture entire flow
```

---

## Security Architecture

### Authentication & Authorization

- **JWT Validation:** API Gateway handles JWT verification at the edge
- **Token Extraction:** From `Authorization: Bearer <token>` header
- **Claims Validation:** Issuer, audience, expiration automatically validated
- **User Context:** JWT claims passed to Lambda for user identification

### Encryption

- **In Transit:**
  - HTTPS only (TLS 1.2+)
  - API Gateway enforces HTTPS
- **At Rest:**
  - DynamoDB encrypted with AWS managed keys
  - Lambda environment variables encrypted

### Network Security

- **No VPC:** Lambda functions run in AWS public network (cost optimization)
- **API Gateway:** Public endpoint with JWT authorization
- **DynamoDB:** VPC endpoints not required (AWS PrivateLink)

### IAM Best Practices

- **Least Privilege:** Lambda roles have minimal permissions
- **Separate Roles:** Each Lambda function has its own IAM role
- **No Hardcoded Credentials:** AWS SDK uses IAM roles automatically

---

## Scalability

### API Gateway

- **Limits:** 10,000 requests/second (burst), unlimited steady-state
- **Auto-scaling:** Automatic, no configuration needed
- **Regional:** Single-region deployment (multi-region possible)

### Lambda

- **Concurrency:** 1,000 concurrent executions per region (default)
- **Reserved Concurrency:** Not configured (cost optimization)
- **Burst Concurrency:** 500-3,000 depending on region
- **Scaling:** Automatic, 1 instance per concurrent request

### DynamoDB

- **Billing Mode:** On-demand (auto-scales to workload)
- **Throughput:** Unlimited read/write capacity
- **Partitions:** Automatically managed by AWS
- **Global Tables:** Not configured (can be added for multi-region)

---

## High Availability & Reliability

### API Gateway

- **Availability:** 99.95% SLA
- **Multi-AZ:** Automatic deployment across multiple availability zones
- **Failover:** Automatic regional failover

### Lambda

- **Availability:** 99.95% SLA
- **Multi-AZ:** Automatic deployment across AZs
- **Retries:** Automatic retries on failure (asynchronous invocations)

### DynamoDB

- **Availability:** 99.99% SLA (single-region)
- **Multi-AZ:** Automatic replication across 3 AZs
- **Backup:** Point-in-time recovery available (not enabled by default)

---

## Cost Optimization Strategies

### 1. Use HTTP API instead of REST API
- **Savings:** 70% cheaper ($1.00/million vs $3.50/million)

### 2. DynamoDB On-Demand Billing
- **Savings:** Pay only for what you use, no idle capacity costs

### 3. Lambda Memory Optimization
- **Savings:** 256 MB default (can be reduced for simple functions)

### 4. Short Log Retention
- **Savings:** 7-day retention (increase for production)

### 5. No NAT Gateway or VPC
- **Savings:** $32-45/month saved by avoiding VPC NAT Gateway

### 6. No Reserved Capacity
- **Savings:** On-demand pricing for low/unpredictable traffic

### 7. Free Tier Utilization
- **Savings:** First year free tier covers most MVP traffic

---

## Monitoring & Observability

### Metrics to Monitor

- **API Gateway:**
  - Count (request volume)
  - IntegrationLatency (Lambda execution time)
  - Latency (total request time)
  - 4XXError, 5XXError (error rates)

- **Lambda:**
  - Invocations (request count)
  - Duration (execution time)
  - Errors (failed invocations)
  - Throttles (concurrency limit reached)
  - ConcurrentExecutions (current active instances)

- **DynamoDB:**
  - ConsumedReadCapacityUnits
  - ConsumedWriteCapacityUnits
  - UserErrors (throttling)

### Alarms to Configure

1. **API Gateway 5XX Errors** > 10 in 5 minutes
2. **Lambda Errors** > 5 in 5 minutes
3. **Lambda Duration** > 9 seconds (approaching timeout)
4. **DynamoDB Throttling** > 0
5. **Cost Anomaly** > $10/day

---

## Disaster Recovery

### Backup Strategy

- **DynamoDB:**
  - Enable point-in-time recovery (production)
  - Manual backups before major changes
  - Retention: 35 days

- **Terraform State:**
  - S3 backend with versioning
  - Cross-region replication (production)

### Recovery Procedures

- **RTO (Recovery Time Objective):** < 1 hour
- **RPO (Recovery Point Objective):** < 5 minutes (with PITR)

### Recovery Steps

1. **DynamoDB Restore:**
   ```bash
   aws dynamodb restore-table-to-point-in-time \
     --source-table-name api-gateway-prod-users \
     --target-table-name api-gateway-prod-users-restored \
     --restore-date-time <timestamp>
   ```

2. **Lambda Rollback:**
   ```bash
   aws lambda update-function-code \
     --function-name api-gateway-prod-user-service \
     --s3-bucket <backup-bucket> \
     --s3-key lambda-backup.zip
   ```

3. **Terraform State Restore:**
   ```bash
   aws s3 cp s3://terraform-state-backup/terraform.tfstate.backup \
             terraform.tfstate
   ```

---

## Future Enhancements

### Phase 1: Enhanced Security
- [ ] AWS WAF integration for DDoS protection
- [ ] API Gateway resource policies
- [ ] Lambda VPC deployment (if required)
- [ ] Secrets Manager for JWT secrets

### Phase 2: Advanced Features
- [ ] API Gateway request/response transformation
- [ ] Lambda layers for shared code
- [ ] Step Functions for complex workflows
- [ ] EventBridge for event-driven architecture

### Phase 3: Multi-Region
- [ ] DynamoDB Global Tables
- [ ] Route53 health checks and failover
- [ ] Multi-region Lambda deployment
- [ ] CloudFront distribution

### Phase 4: DevOps
- [ ] CI/CD pipeline (GitHub Actions, GitLab CI)
- [ ] Automated testing (integration, load)
- [ ] Blue-green deployments
- [ ] Canary releases

---

## References

- [AWS API Gateway HTTP API](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api.html)
- [AWS Lambda Best Practices](https://docs.aws.amazon.com/lambda/latest/dg/best-practices.html)
- [DynamoDB Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/best-practices.html)
- [AWS Well-Architected Framework](https://aws.amazon.com/architecture/well-architected/)
