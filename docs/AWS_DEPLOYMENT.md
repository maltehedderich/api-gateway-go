# AWS Serverless Deployment Guide

This guide explains how to deploy the API Gateway to AWS using a fully serverless, cost-optimized architecture.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Prerequisites](#prerequisites)
3. [Quick Start](#quick-start)
4. [Step-by-Step Deployment](#step-by-step-deployment)
5. [Configuration](#configuration)
6. [Testing](#testing)
7. [Monitoring](#monitoring)
8. [Troubleshooting](#troubleshooting)
9. [Cost Optimization](#cost-optimization)
10. [Cleanup](#cleanup)

## Architecture Overview

### Serverless Components

```
┌──────────┐
│  Client  │
└────┬─────┘
     │
     ▼
┌────────────────────┐
│  API Gateway       │  $1/million requests (after free tier)
│  (HTTP API)        │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│  AWS Lambda        │  1M requests/month free
│  (Container Image) │  400K GB-seconds free
└────────┬───────────┘
         │
         ├──────────────────┐
         │                  │
         ▼                  ▼
┌────────────────┐  ┌──────────────┐
│  DynamoDB      │  │  Backend     │
│  (On-Demand)   │  │  Services    │
└────────┬───────┘  └──────────────┘
         │
         ▼
┌────────────────┐
│  CloudWatch    │  5GB free ingestion/month
│  Logs          │
└────────────────┘
```

### Key Features

- **Cost-Optimized**: Pay only for what you use, ~$0-1/month for MVP traffic
- **Auto-Scaling**: Lambda scales automatically from 0 to thousands of requests/second
- **No Infrastructure Management**: Fully managed by AWS
- **High Availability**: Multi-AZ by default
- **Low Latency**: DynamoDB provides single-digit millisecond response times

### Trade-offs

| Aspect | Traditional EC2/ECS | Serverless Lambda |
|--------|---------------------|-------------------|
| **Cost** | ~$10-50/month minimum | $0-1/month for low traffic |
| **Cold Start** | None | 1-2 seconds (container) |
| **Scaling** | Manual/auto-scaling group | Automatic, instant |
| **Maintenance** | OS patches, updates | None |
| **Warm Response Time** | <10ms | 10-50ms (Lambda overhead) |

## Prerequisites

### Required Tools

1. **AWS CLI** version 2.0 or later
   ```bash
   # Install on macOS
   brew install awscli

   # Install on Linux
   curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
   unzip awscliv2.zip
   sudo ./aws/install

   # Verify installation
   aws --version
   ```

2. **Terraform** version 1.5.0 or later
   ```bash
   # Install on macOS
   brew tap hashicorp/tap
   brew install hashicorp/tap/terraform

   # Install on Linux
   wget https://releases.hashicorp.com/terraform/1.5.0/terraform_1.5.0_linux_amd64.zip
   unzip terraform_1.5.0_linux_amd64.zip
   sudo mv terraform /usr/local/bin/

   # Verify installation
   terraform version
   ```

3. **Docker** (for building Lambda container image)
   ```bash
   # Verify installation
   docker --version
   ```

### AWS Account Setup

1. **Create AWS Account** (if you don't have one)
   - Go to https://aws.amazon.com/
   - Follow the sign-up process
   - Note: Requires credit card for verification

2. **Configure AWS Credentials**
   ```bash
   aws configure
   # Enter:
   # - AWS Access Key ID
   # - AWS Secret Access Key
   # - Default region: us-east-1
   # - Default output format: json
   ```

3. **Verify Credentials**
   ```bash
   aws sts get-caller-identity
   ```

### Required IAM Permissions

Your IAM user needs the following permissions:
- Lambda full access
- API Gateway full access
- DynamoDB full access
- IAM role creation and management
- CloudWatch Logs
- ECR (Elastic Container Registry)
- ACM & Route53 (if using custom domain)

## Quick Start

For experienced users, here's the TL;DR:

```bash
# 1. Build and push Docker image
cd terraform
./build-and-push.sh

# 2. Configure Terraform
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your configuration

# 3. Deploy infrastructure
terraform init
terraform plan
terraform apply

# 4. Test
curl $(terraform output -raw api_endpoint)/_health/live
```

## Step-by-Step Deployment

### Step 1: Build Lambda Container Image

Navigate to the terraform directory:

```bash
cd terraform
```

Run the build and push script:

```bash
./build-and-push.sh
```

This script will:
1. Build the Docker image for Lambda
2. Create an ECR repository (if it doesn't exist)
3. Login to ECR
4. Tag the image
5. Push the image to ECR

**Expected output:**
```
==> Building Docker image for Lambda...
    Image URI: 123456789012.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest
==> Docker image built successfully
==> Logging in to ECR...
Login Succeeded
==> Ensuring ECR repository exists...
==> Pushing image to ECR...
latest: digest: sha256:abc123... size: 1234
==> Image pushed successfully!

Next steps:
  1. Update terraform/terraform.tfvars with:
     lambda_image_uri = "123456789012.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest"
  2. Run: terraform apply
```

**Copy the image URI** from the output - you'll need it in the next step.

### Step 2: Configure Terraform Variables

Create your `terraform.tfvars` file:

```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your configuration:

```hcl
# AWS Configuration
aws_region   = "us-east-1"
environment  = "dev"
project_name = "api-gateway"

# Lambda Configuration
# PASTE THE IMAGE URI FROM STEP 1 HERE:
lambda_image_uri = "123456789012.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest"
lambda_memory_size      = 512
lambda_timeout          = 30
lambda_log_retention_days = 7

# DynamoDB Configuration
dynamodb_billing_mode = "PAY_PER_REQUEST"  # Most cost-effective for low traffic

# Backend Service URLs
# IMPORTANT: Replace with your actual backend service URLs
backend_users_url  = "https://users-api.example.com"
backend_orders_url = "https://orders-api.example.com"

# Gateway Configuration
log_level            = "info"
enable_authorization = false  # Set to true if you want JWT auth
enable_rate_limiting = true   # Keep enabled for DDoS protection

# Additional Tags
tags = {
  Owner      = "DevOps Team"
  CostCenter = "Engineering"
}
```

### Step 3: Initialize Terraform

Initialize Terraform to download required providers:

```bash
terraform init
```

**Expected output:**
```
Initializing modules...
Initializing the backend...
Initializing provider plugins...
- Installing hashicorp/aws v5.x.x...

Terraform has been successfully initialized!
```

### Step 4: Preview Changes

Review what Terraform will create:

```bash
terraform plan
```

This will show you all resources to be created:
- 1 DynamoDB table
- 3 IAM roles/policies
- 1 Lambda function
- 1 CloudWatch Log Group for Lambda
- 1 API Gateway HTTP API
- 1 API Gateway stage
- 1 CloudWatch Log Group for API Gateway

**Total: ~10 resources**

### Step 5: Deploy Infrastructure

Apply the Terraform configuration:

```bash
terraform apply
```

Type `yes` when prompted.

**Deployment time:** ~2-5 minutes

**Expected output:**
```
Apply complete! Resources: 10 added, 0 changed, 0 destroyed.

Outputs:

api_endpoint = "https://abc123xyz.execute-api.us-east-1.amazonaws.com"
api_id = "abc123xyz"
deployment_summary = {
  "api_endpoint" = "https://abc123xyz.execute-api.us-east-1.amazonaws.com"
  "authorization" = "disabled"
  "custom_domain" = "disabled"
  "dynamodb_table" = "api-gateway-dev-rate-limits"
  "environment" = "dev"
  "lambda_function" = "api-gateway-dev-gateway"
  "log_level" = "info"
  "rate_limiting" = "enabled"
  "region" = "us-east-1"
}
dynamodb_table_arn = "arn:aws:dynamodb:us-east-1:123456789012:table/api-gateway-dev-rate-limits"
dynamodb_table_name = "api-gateway-dev-rate-limits"
lambda_function_arn = "arn:aws:lambda:us-east-1:123456789012:function:api-gateway-dev-gateway"
lambda_function_name = "api-gateway-dev-gateway"
lambda_log_group = "/aws/lambda/api-gateway-dev-gateway"
```

### Step 6: Save the API Endpoint

Save the API endpoint URL:

```bash
export API_ENDPOINT=$(terraform output -raw api_endpoint)
echo $API_ENDPOINT
```

## Testing

### Test Health Endpoint

```bash
curl $API_ENDPOINT/_health/live
```

**Expected response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-11-17T12:00:00Z",
  "checks": {
    "config": "healthy"
  }
}
```

### Test Readiness

```bash
curl $API_ENDPOINT/_health/ready
```

**Expected response:**
```json
{
  "status": "ready",
  "timestamp": "2025-11-17T12:00:00Z",
  "checks": {
    "config": "healthy",
    "dynamodb": "healthy"
  }
}
```

### Test API Routes

**Users endpoint:**
```bash
curl $API_ENDPOINT/api/v1/users
```

**Orders endpoint:**
```bash
curl $API_ENDPOINT/api/v1/orders
```

### Test Rate Limiting

Send multiple requests to test rate limiting:

```bash
for i in {1..10}; do
  curl -i $API_ENDPOINT/api/v1/users
  sleep 0.1
done
```

Look for rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1700000000
```

## Monitoring

### CloudWatch Logs

**View Lambda logs:**
```bash
# Stream live logs
aws logs tail /aws/lambda/api-gateway-dev-gateway --follow

# View last 10 minutes
aws logs tail /aws/lambda/api-gateway-dev-gateway --since 10m
```

**View API Gateway logs:**
```bash
aws logs tail /aws/apigateway/api-gateway-dev-api --follow
```

### CloudWatch Metrics

**Lambda metrics:**
```bash
# Invocations
aws cloudwatch get-metric-statistics \
  --namespace AWS/Lambda \
  --metric-name Invocations \
  --dimensions Name=FunctionName,Value=api-gateway-dev-gateway \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum

# Duration
aws cloudwatch get-metric-statistics \
  --namespace AWS/Lambda \
  --metric-name Duration \
  --dimensions Name=FunctionName,Value=api-gateway-dev-gateway \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Average,Maximum
```

### DynamoDB Metrics

```bash
aws cloudwatch get-metric-statistics \
  --namespace AWS/DynamoDB \
  --metric-name ConsumedReadCapacityUnits \
  --dimensions Name=TableName,Value=api-gateway-dev-rate-limits \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum
```

## Configuration

### Environment Variables

The Lambda function can be configured via environment variables in `terraform.tfvars`:

```hcl
# In terraform/main.tf, add to environment_variables:
GATEWAY_LOG_LEVEL          = "debug"           # debug, info, warn, error
GATEWAY_AUTH_ENABLED       = "true"            # Enable JWT auth
GATEWAY_RATELIMIT_ENABLED  = "true"            # Enable rate limiting
GATEWAY_BACKEND_USERS_URL  = "https://..."     # Backend URLs
```

After changes, re-apply Terraform:
```bash
terraform apply
```

### Update Lambda Code

To deploy code changes without Terraform:

1. Build and push new image:
   ```bash
   ./build-and-push.sh
   ```

2. Update Lambda function:
   ```bash
   aws lambda update-function-code \
     --function-name api-gateway-dev-gateway \
     --image-uri 123456789012.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest
   ```

## Troubleshooting

### Issue: Lambda Cold Starts

**Symptom:** First request takes 1-2 seconds

**Solution:**
- Increase memory (more CPU allocated): `lambda_memory_size = 1024`
- Enable provisioned concurrency (costs more):
  ```bash
  aws lambda put-provisioned-concurrency-config \
    --function-name api-gateway-dev-gateway \
    --provisioned-concurrent-executions 1 \
    --qualifier $LATEST
  ```

### Issue: Rate Limiting Not Working

**Check DynamoDB table exists:**
```bash
aws dynamodb describe-table --table-name api-gateway-dev-rate-limits
```

**Check Lambda IAM permissions:**
```bash
aws iam get-role-policy \
  --role-name api-gateway-dev-lambda-role \
  --policy-name api-gateway-dev-dynamodb-access
```

### Issue: Backend Connection Errors

**Check Lambda logs:**
```bash
aws logs tail /aws/lambda/api-gateway-dev-gateway --follow
```

**Verify backend URLs are reachable:**
```bash
curl -v https://your-backend-url.com
```

### Issue: 502 Bad Gateway

**Check Lambda function status:**
```bash
aws lambda get-function --function-name api-gateway-dev-gateway
```

**Check recent errors:**
```bash
aws logs filter-log-events \
  --log-group-name /aws/lambda/api-gateway-dev-gateway \
  --filter-pattern "ERROR"
```

## Cost Optimization

### Current Cost Estimate

**For 10,000 requests/month:**
- API Gateway: $0.01 (within free tier)
- Lambda: $0 (within free tier)
- DynamoDB: $0 (within free tier)
- **Total: ~$0/month**

**For 1,000,000 requests/month:**
- API Gateway: ~$1.00
- Lambda: ~$1.50 (assuming 512MB, 100ms avg)
- DynamoDB: ~$1.25 (assuming 5 WCU/RCU average)
- **Total: ~$3.75/month**

### Optimization Tips

1. **Use On-Demand Pricing** for DynamoDB (default in config)
2. **Right-Size Lambda Memory**: Start with 512MB, adjust based on metrics
3. **Reduce Log Retention**: Use 7 days for dev, 30 days for prod
4. **Enable Compression**: Already configured via Lambda Web Adapter
5. **Monitor Costs**: Set up AWS Budget alerts

### Set Up Cost Alerts

```bash
aws budgets create-budget \
  --account-id $(aws sts get-caller-identity --query Account --output text) \
  --budget file://budget.json

# budget.json:
{
  "BudgetName": "api-gateway-monthly",
  "BudgetType": "COST",
  "TimeUnit": "MONTHLY",
  "BudgetLimit": {
    "Amount": "10",
    "Unit": "USD"
  }
}
```

## Cleanup

To delete all resources and stop incurring costs:

```bash
terraform destroy
```

Type `yes` when prompted.

**⚠️ WARNING:** This will permanently delete:
- Lambda function
- API Gateway
- DynamoDB table and all data
- CloudWatch Logs
- IAM roles
- ECR images (must be deleted manually)

**To also delete ECR repository:**
```bash
aws ecr delete-repository \
  --repository-name api-gateway \
  --region us-east-1 \
  --force
```

**Estimated time:** 2-3 minutes

---

## Next Steps

- [Configure Custom Domain](./CUSTOM_DOMAIN.md)
- [Enable JWT Authorization](./AUTHORIZATION.md)
- [Set Up CI/CD Pipeline](./CICD.md)
- [Production Best Practices](./PRODUCTION.md)

## Support

For issues or questions:
- Check [Troubleshooting](#troubleshooting) section
- Review [Terraform README](../terraform/README.md)
- See main project [README](../README.md)
