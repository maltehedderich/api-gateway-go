# API Gateway - AWS Serverless Terraform Deployment

This Terraform configuration deploys a **production-ready, cost-optimized, serverless API Gateway** on AWS using native AWS services.

## Architecture Overview

```
Internet
   ↓
AWS API Gateway HTTP API (JWT Auth + Throttling)
   ↓
   ├→ Lambda: user-service (CRUD operations on DynamoDB users table)
   ├→ Lambda: order-service (CRUD operations on DynamoDB orders table)
   ├→ Lambda: admin-service (Admin operations, JWT required)
   └→ Lambda: status-service (Health checks, public access)
   ↓
DynamoDB Tables (on-demand billing, encrypted)
   ↓
CloudWatch Logs (7-day retention)
```

### Key Features

✅ **Fully Serverless** - API Gateway HTTP API + Lambda + DynamoDB
✅ **Cost Optimized** - Pay-per-use, Free Tier eligible (~$0-10/month for low traffic)
✅ **JWT Authorization** - Native API Gateway JWT authorizer (RS256/HS256)
✅ **Rate Limiting** - Per-route and global throttling
✅ **Auto-scaling** - Lambda concurrency, DynamoDB on-demand
✅ **Encrypted** - DynamoDB encryption at rest, HTTPS only
✅ **Observability** - CloudWatch Logs, X-Ray tracing
✅ **Multi-environment** - Dev/staging/prod with variables
✅ **Custom Domain** - Optional ACM + Route53 integration
✅ **CORS Enabled** - Configurable CORS support

---

## Prerequisites

### Required Tools

- **Terraform** >= 1.5.0 ([Install](https://developer.hashicorp.com/terraform/downloads))
- **AWS CLI** >= 2.0 ([Install](https://aws.amazon.com/cli/))
- **Go** >= 1.21 (for building Lambda functions) ([Install](https://go.dev/dl/))
- **Make** (optional, for convenience commands)

### AWS Account Setup

1. **AWS Account** with appropriate permissions
2. **AWS Credentials** configured:
   ```bash
   aws configure
   # OR set environment variables:
   export AWS_ACCESS_KEY_ID="your-access-key"
   export AWS_SECRET_ACCESS_KEY="your-secret-key"
   export AWS_DEFAULT_REGION="us-east-1"
   ```

3. **IAM Permissions** required:
   - API Gateway (create, update, delete)
   - Lambda (create, update, delete)
   - DynamoDB (create, update, delete)
   - IAM (create roles and policies)
   - CloudWatch Logs (create log groups)
   - ACM (optional, for custom domains)
   - Route53 (optional, for custom domains)

---

## Quick Start

### 1. Build Lambda Functions

Before deploying, build the Lambda function binaries:

```bash
cd terraform
./scripts/build-lambdas.sh
```

This creates `bootstrap` executables in each `lambda-src/*/` directory.

### 2. Configure Variables

Copy the example configuration:

```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your settings:

```hcl
# Required Variables
aws_region   = "us-east-1"
environment  = "dev"
project_name = "api-gateway"

# JWT Configuration (REQUIRED for JWT authorizer)
enable_jwt_authorizer = true
jwt_issuer           = "https://your-auth-service.com"
jwt_audience         = ["api-gateway"]

# Optional: Custom Domain
enable_custom_domain = false
# domain_name         = "api.example.com"
# certificate_arn     = "arn:aws:acm:us-east-1:123456789012:certificate/..."
# route53_zone_id     = "Z1234567890ABC"
```

### 3. Initialize Terraform

```bash
terraform init
```

### 4. Plan Deployment

Preview the infrastructure changes:

```bash
terraform plan
```

### 5. Deploy

Apply the configuration:

```bash
terraform apply
```

Review the plan and type `yes` to confirm.

### 6. Get API Endpoint

After deployment, Terraform outputs the API endpoint:

```bash
terraform output api_endpoint
# Output: https://abcdef1234.execute-api.us-east-1.amazonaws.com
```

### 7. Test API

Test the health check endpoint (no auth required):

```bash
API_ENDPOINT=$(terraform output -raw api_endpoint)
curl $API_ENDPOINT/_health
```

Test authenticated endpoints (JWT required):

```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     $API_ENDPOINT/api/v1/users
```

---

## Configuration Reference

### Core Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `aws_region` | AWS region for deployment | `us-east-1` | No |
| `environment` | Environment name (dev/staging/prod) | `dev` | No |
| `project_name` | Project name for resource naming | `api-gateway` | No |

### API Gateway Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `api_gateway_throttle_burst_limit` | Max concurrent requests | `100` | No |
| `api_gateway_throttle_rate_limit` | Requests per second | `50` | No |
| `enable_api_gateway_logs` | Enable API Gateway access logs | `true` | No |
| `api_gateway_log_retention_days` | Log retention in days | `7` | No |

### JWT Authorization Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `enable_jwt_authorizer` | Enable JWT authorizer | `true` | No |
| `jwt_issuer` | JWT issuer URL | `https://auth.example.com` | **Yes** (if JWT enabled) |
| `jwt_audience` | JWT audience list | `["api-gateway"]` | **Yes** (if JWT enabled) |
| `jwt_identity_source` | JWT token source | `$request.header.Authorization` | No |

### Lambda Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `lambda_runtime` | Lambda runtime | `go1.x` | No |
| `lambda_memory_size` | Memory in MB | `256` | No |
| `lambda_timeout` | Timeout in seconds | `10` | No |
| `lambda_log_retention_days` | Log retention in days | `7` | No |
| `lambda_environment_variables` | Environment variables | `{}` | No |

### DynamoDB Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `enable_dynamodb` | Enable DynamoDB tables | `true` | No |
| `dynamodb_billing_mode` | Billing mode | `PAY_PER_REQUEST` | No |
| `dynamodb_point_in_time_recovery` | Enable PITR | `false` | No |
| `dynamodb_deletion_protection` | Enable deletion protection | `false` | No |
| `dynamodb_tables` | Table configurations | See below | No |

### Custom Domain Variables (Optional)

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `enable_custom_domain` | Enable custom domain | `false` | No |
| `domain_name` | Custom domain name | `""` | Yes (if custom domain enabled) |
| `certificate_arn` | ACM certificate ARN | `""` | Yes (if custom domain enabled) |
| `route53_zone_id` | Route53 hosted zone ID | `""` | Yes (if custom domain enabled) |

### CORS Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `enable_cors` | Enable CORS | `true` | No |
| `cors_allow_origins` | Allowed origins | `["*"]` | No |
| `cors_allow_methods` | Allowed HTTP methods | `["GET", "POST", "PUT", "DELETE", "OPTIONS"]` | No |
| `cors_allow_headers` | Allowed headers | `["Content-Type", "Authorization", ...]` | No |

---

## DynamoDB Table Configuration

The default configuration creates two tables: `users` and `orders`.

### Users Table

```hcl
users = {
  hash_key = "user_id"
  attributes = [
    { name = "user_id", type = "S" },
    { name = "email", type = "S" }
  ]
  global_secondary_indexes = [
    {
      name            = "email-index"
      hash_key        = "email"
      projection_type = "ALL"
    }
  ]
}
```

### Orders Table

```hcl
orders = {
  hash_key  = "order_id"
  range_key = "created_at"
  attributes = [
    { name = "order_id", type = "S" },
    { name = "created_at", type = "N" },
    { name = "user_id", type = "S" }
  ]
  global_secondary_indexes = [
    {
      name            = "user-orders-index"
      hash_key        = "user_id"
      range_key       = "created_at"
      projection_type = "ALL"
    }
  ]
}
```

### Customizing Tables

To add or modify tables, edit `dynamodb_tables` in `terraform.tfvars`:

```hcl
dynamodb_tables = {
  my_custom_table = {
    hash_key = "id"
    attributes = [
      { name = "id", type = "S" }
    ]
  }
}
```

---

## API Routes

The deployment creates the following routes:

### User Service Routes (JWT Required)

- `GET /api/v1/users` - List all users
- `POST /api/v1/users` - Create a new user
- `GET /api/v1/users/{id}` - Get user by ID
- `PUT /api/v1/users/{id}` - Update user
- `DELETE /api/v1/users/{id}` - Delete user

### Order Service Routes (JWT Required)

- `GET /api/v1/orders` - List all orders
- `POST /api/v1/orders` - Create a new order
- `GET /api/v1/orders/{id}` - Get order by ID
- `PUT /api/v1/orders/{id}` - Update order status
- `DELETE /api/v1/orders/{id}` - Delete order

### Admin Service Routes (JWT Required)

- `GET /api/v1/admin` - Get admin statistics
- `POST /api/v1/admin` - Admin operations

### Status Service Routes (Public, No Auth)

- `GET /api/v1/public/health` - Public health check
- `GET /_health` - System health check
- `GET /_health/ready` - Readiness probe
- `GET /_health/live` - Liveness probe

---

## JWT Configuration

### JWT Issuer Setup

You need a JWT issuer service that provides:

1. **JWKS endpoint** (for RS256 public keys)
2. **Token issuance** with required claims

### Required JWT Claims

- `iss` (issuer) - Must match `jwt_issuer` variable
- `aud` (audience) - Must be in `jwt_audience` list
- `sub` (subject) - User ID
- `exp` (expiration) - Token expiration timestamp

### Example JWT Token

```json
{
  "iss": "https://your-auth-service.com",
  "aud": ["api-gateway"],
  "sub": "user-123",
  "exp": 1700000000,
  "iat": 1699999000,
  "roles": ["user", "admin"]
}
```

### Testing with Mock JWT

For development, use [jwt.io](https://jwt.io) to generate test tokens:

1. Set `jwt_issuer` to your auth service URL
2. Generate a token with matching issuer and audience
3. Use `HS256` algorithm with a shared secret (update authorizer config)

---

## Custom Domain Setup

### Prerequisites

1. **Domain name** registered in Route53 or external registrar
2. **ACM certificate** in the **same region** as API Gateway
3. **Route53 hosted zone** (if using Route53 for DNS)

### Step 1: Request ACM Certificate

```bash
aws acm request-certificate \
  --domain-name api.example.com \
  --validation-method DNS \
  --region us-east-1
```

### Step 2: Validate Certificate

Follow DNS validation instructions in ACM console or:

```bash
aws acm describe-certificate \
  --certificate-arn arn:aws:acm:us-east-1:123456789012:certificate/...
```

### Step 3: Configure Terraform

Update `terraform.tfvars`:

```hcl
enable_custom_domain = true
domain_name         = "api.example.com"
certificate_arn     = "arn:aws:acm:us-east-1:123456789012:certificate/..."
route53_zone_id     = "Z1234567890ABC"
```

### Step 4: Apply

```bash
terraform apply
```

Your API will be accessible at `https://api.example.com`.

---

## Monitoring and Observability

### CloudWatch Logs

Logs are stored in CloudWatch Logs with 7-day retention (configurable):

- **API Gateway Logs**: `/aws/apigateway/api-gateway-{environment}`
- **Lambda Logs**: `/aws/lambda/api-gateway-{environment}-{service-name}`

View logs:

```bash
# API Gateway logs
aws logs tail /aws/apigateway/api-gateway-dev --follow

# Lambda logs
aws logs tail /aws/lambda/api-gateway-dev-user-service --follow
```

### CloudWatch Metrics

API Gateway and Lambda automatically publish metrics to CloudWatch:

- **API Gateway**: Request count, latency, 4xx/5xx errors
- **Lambda**: Invocations, duration, errors, throttles
- **DynamoDB**: Consumed capacity, throttled requests

### X-Ray Tracing

Lambda functions have X-Ray tracing enabled by default. View traces in AWS X-Ray console.

### Cost Monitoring

Enable AWS Cost Explorer and set up billing alerts:

```bash
aws budgets create-budget --cli-input-json file://budget.json
```

---

## Cost Estimation

### Free Tier (First 12 Months)

- **API Gateway HTTP API**: 1M requests/month
- **Lambda**: 1M requests + 400K GB-seconds compute
- **DynamoDB**: 25 GB storage, 25 WCU, 25 RCU
- **CloudWatch Logs**: 5 GB ingestion + 5 GB storage

### Estimated Monthly Cost (After Free Tier)

**Low Traffic (10K requests/month)**:
- API Gateway: $0.01 (10K requests × $1/million)
- Lambda: $0.00 (within free tier)
- DynamoDB: $0.00 (on-demand, minimal usage)
- CloudWatch: $0.00 (within free tier)
- **Total: ~$0-1/month**

**Medium Traffic (100K requests/month)**:
- API Gateway: $0.10
- Lambda: $0.20 (100K × 256MB × 100ms)
- DynamoDB: $1.25 (on-demand writes)
- CloudWatch: $0.50
- **Total: ~$2-3/month**

**High Traffic (1M requests/month)**:
- API Gateway: $1.00
- Lambda: $2.00
- DynamoDB: $12.50
- CloudWatch: $5.00
- **Total: ~$20-25/month**

---

## Development Workflow

### Local Development

1. **Build Lambda functions**:
   ```bash
   ./scripts/build-lambdas.sh
   ```

2. **Test locally** (optional, requires SAM CLI):
   ```bash
   sam local start-api
   ```

3. **Deploy to AWS**:
   ```bash
   terraform apply
   ```

### Adding a New Lambda Function

1. **Create directory**: `lambda-src/my-service/`
2. **Add Go code**: `lambda-src/my-service/main.go`
3. **Add go.mod**: `lambda-src/my-service/go.mod`
4. **Update `main.tf`**: Add to `lambda_functions` local
5. **Update routes**: Add to `api_routes` local
6. **Build and deploy**:
   ```bash
   ./scripts/build-lambdas.sh
   terraform apply
   ```

### Multi-Environment Deployment

Use Terraform workspaces or separate state files:

**Option 1: Workspaces**
```bash
terraform workspace new staging
terraform workspace select staging
terraform apply -var="environment=staging"
```

**Option 2: Separate Directories**
```bash
cp -r terraform terraform-staging
cd terraform-staging
terraform init
terraform apply -var="environment=staging"
```

---

## Terraform Backend Configuration

By default, Terraform uses local state. For production, use remote state in S3:

### Step 1: Create S3 Bucket and DynamoDB Table

```bash
aws s3 mb s3://your-terraform-state-bucket
aws dynamodb create-table \
  --table-name terraform-state-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```

### Step 2: Update `versions.tf`

Uncomment the backend configuration:

```hcl
terraform {
  backend "s3" {
    bucket         = "your-terraform-state-bucket"
    key            = "api-gateway/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-state-locks"
  }
}
```

### Step 3: Initialize Backend

```bash
terraform init -migrate-state
```

---

## Troubleshooting

### Lambda Build Fails

**Error**: `go: command not found`
**Solution**: Install Go >= 1.21 from https://go.dev/dl/

**Error**: `build failed for user-service`
**Solution**: Check `lambda-src/user-service/main.go` for syntax errors

### JWT Authorization Fails

**Error**: `{"message":"Unauthorized"}`
**Solution**:
1. Verify JWT issuer matches `jwt_issuer` variable
2. Verify JWT audience is in `jwt_audience` list
3. Check token expiration
4. View CloudWatch logs for detailed error

### DynamoDB Access Denied

**Error**: `AccessDeniedException`
**Solution**: Check Lambda IAM role has DynamoDB permissions (automatically granted by Terraform)

### API Gateway 429 Too Many Requests

**Error**: Rate limit exceeded
**Solution**: Increase `api_gateway_throttle_rate_limit` or `api_gateway_throttle_burst_limit`

### Custom Domain Not Working

**Error**: Certificate validation pending
**Solution**: Complete DNS validation for ACM certificate

**Error**: DNS not resolving
**Solution**: Wait for Route53 DNS propagation (up to 48 hours)

---

## Security Best Practices

### 1. Enable Deletion Protection (Production)

```hcl
dynamodb_deletion_protection = true
```

### 2. Enable Point-in-Time Recovery (Production)

```hcl
dynamodb_point_in_time_recovery = true
```

### 3. Use Secrets Manager for Sensitive Data

```hcl
lambda_environment_variables = {
  DB_PASSWORD = data.aws_secretsmanager_secret_version.db_password.secret_string
}
```

### 4. Restrict CORS Origins (Production)

```hcl
cors_allow_origins = ["https://example.com"]
```

### 5. Use VPC for Lambda (Optional)

For enhanced security, deploy Lambda in a VPC with private subnets and NAT Gateway.

### 6. Enable AWS WAF (Production)

Add AWS WAF to API Gateway for DDoS protection and SQL injection prevention.

---

## Cleanup

To destroy all resources:

```bash
terraform destroy
```

**⚠️ WARNING**: This will delete all data in DynamoDB tables. Backup data before destroying.

---

## Module Structure

```
terraform/
├── main.tf                    # Root module
├── variables.tf               # Input variables
├── outputs.tf                 # Outputs
├── versions.tf                # Provider versions
├── terraform.tfvars.example   # Example configuration
├── README.md                  # This file
│
├── modules/                   # Terraform modules
│   ├── api-gateway/          # API Gateway HTTP API
│   ├── lambda/               # Lambda function
│   ├── dynamodb/             # DynamoDB table
│   └── custom-domain/        # Custom domain (optional)
│
├── lambda-src/               # Lambda function source code
│   ├── user-service/
│   ├── order-service/
│   ├── admin-service/
│   └── status-service/
│
└── scripts/
    └── build-lambdas.sh      # Build script for Lambda functions
```

---

## References

- [AWS API Gateway HTTP API](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api.html)
- [AWS Lambda](https://docs.aws.amazon.com/lambda/)
- [DynamoDB](https://docs.aws.amazon.com/dynamodb/)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
- [Go Lambda Runtime](https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html)

---

## Support

For issues and questions:

1. Check the [Troubleshooting](#troubleshooting) section
2. Review [CloudWatch Logs](#cloudwatch-logs)
3. Open an issue in the repository

---

## License

[Your License Here]
