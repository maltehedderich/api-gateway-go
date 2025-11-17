# API Gateway - AWS Serverless Deployment with Terraform

This directory contains Terraform Infrastructure as Code (IaC) for deploying the API Gateway on AWS using a fully serverless, cost-optimized architecture.

## Architecture Overview

```
┌─────────┐      ┌──────────────────┐      ┌────────────────┐      ┌──────────────┐
│ Client  │─────▶│  API Gateway     │─────▶│  Lambda        │─────▶│  Backend     │
│         │      │  (HTTP API)      │      │  (Container)   │      │  Services    │
└─────────┘      └──────────────────┘      └────────────────┘      └──────────────┘
                                                    │
                                                    ▼
                                            ┌──────────────┐
                                            │  DynamoDB    │
                                            │ (Rate Limits)│
                                            └──────────────┘
                                                    │
                                                    ▼
                                            ┌──────────────┐
                                            │ CloudWatch   │
                                            │   Logs       │
                                            └──────────────┘
```

### Components

| Component | Purpose | Cost |
|-----------|---------|------|
| **API Gateway HTTP API** | Entry point, HTTP routing | $1/million requests (after free tier) |
| **Lambda (Container)** | Runs Go gateway application | Free tier: 1M requests/month |
| **DynamoDB** | Stores rate limiting state | On-demand: Pay per request (free tier available) |
| **CloudWatch Logs** | Logging and monitoring | 5GB free ingestion/month |
| **IAM** | Security and permissions | Free |
| **ACM + Route53** (Optional) | Custom domain & SSL | Route53: $0.50/month |

**Estimated Monthly Cost (10K requests):** $0 - $0.50
**Estimated Monthly Cost (100K requests):** $0.50 - $1.50

## Prerequisites

### Required Tools

- **Terraform** >= 1.5.0 ([Install](https://www.terraform.io/downloads))
- **AWS CLI** >= 2.0 ([Install](https://aws.amazon.com/cli/))
- **Docker** (for building Lambda container image)
- **Make** (optional, for convenience commands)

### AWS Account Setup

1. **AWS Account** with appropriate permissions
2. **AWS Credentials** configured locally:
   ```bash
   aws configure
   # Or use environment variables:
   export AWS_ACCESS_KEY_ID="your-access-key"
   export AWS_SECRET_ACCESS_KEY="your-secret-key"
   export AWS_DEFAULT_REGION="us-east-1"
   ```

3. **IAM Permissions** required:
   - Lambda full access
   - API Gateway full access
   - DynamoDB full access
   - IAM role creation
   - CloudWatch Logs
   - ECR (for container registry)
   - ACM & Route53 (if using custom domain)

### ECR Repository

Before deploying, create an ECR repository and push your Docker image:

```bash
# Create ECR repository
aws ecr create-repository --repository-name api-gateway --region us-east-1

# Get ECR login
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin \
  <account-id>.dkr.ecr.us-east-1.amazonaws.com

# Build and tag image
docker build -t api-gateway:latest -f Dockerfile.lambda .
docker tag api-gateway:latest \
  <account-id>.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest

# Push image
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest
```

## Quick Start

### 1. Configure Variables

Copy the example variables file and customize:

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your configuration:

```hcl
aws_region        = "us-east-1"
environment       = "dev"
backend_users_url = "https://your-users-service.com"
backend_orders_url = "https://your-orders-service.com"
```

**⚠️ IMPORTANT:** Update the Lambda image URI in `terraform.tfvars`:

```hcl
# Add this to terraform.tfvars or pass as variable
# lambda_image_uri = "<account-id>.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest"
```

Since the Lambda module has `image_uri` as a variable, you need to either:
- Add it to `variables.tf` in the root module and pass it through
- Or update the Lambda module to pull from ECR

### 2. Initialize Terraform

```bash
terraform init
```

This downloads required providers and initializes the backend.

### 3. Plan Deployment

```bash
terraform plan
```

Review the planned changes. Terraform will show all resources to be created.

### 4. Deploy Infrastructure

```bash
terraform apply
```

Type `yes` when prompted. Deployment takes ~2-5 minutes.

### 5. Get API Endpoint

```bash
terraform output api_endpoint
```

Example output:
```
api_endpoint = "https://abc123xyz.execute-api.us-east-1.amazonaws.com"
```

### 6. Test Your API

```bash
# Health check
curl https://your-api-endpoint/_health/live

# Test routes
curl https://your-api-endpoint/api/v1/users
curl https://your-api-endpoint/api/v1/orders
```

## Directory Structure

```
terraform/
├── main.tf                 # Root module configuration
├── variables.tf            # Input variables
├── outputs.tf              # Output values
├── terraform.tfvars.example # Example variable values
├── .gitignore              # Git ignore rules
├── README.md               # This file
└── modules/                # Reusable Terraform modules
    ├── lambda/             # Lambda function module
    │   ├── main.tf
    │   ├── variables.tf
    │   └── outputs.tf
    ├── api-gateway/        # API Gateway module
    │   ├── main.tf
    │   ├── variables.tf
    │   └── outputs.tf
    ├── dynamodb/           # DynamoDB table module
    │   ├── main.tf
    │   ├── variables.tf
    │   └── outputs.tf
    ├── iam/                # IAM roles and policies module
    │   ├── main.tf
    │   ├── variables.tf
    │   └── outputs.tf
    └── custom-domain/      # Custom domain module (optional)
        ├── main.tf
        ├── variables.tf
        └── outputs.tf
```

## Configuration Guide

### Required Variables

These must be configured in `terraform.tfvars`:

| Variable | Description | Example |
|----------|-------------|---------|
| `aws_region` | AWS region for deployment | `us-east-1` |
| `backend_users_url` | Users service backend URL | `https://users.example.com` |
| `backend_orders_url` | Orders service backend URL | `https://orders.example.com` |

### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `environment` | `dev` | Environment name (dev, staging, prod) |
| `project_name` | `api-gateway` | Project name for resource naming |
| `lambda_memory_size` | `512` | Lambda memory in MB (128-10240) |
| `lambda_timeout` | `30` | Lambda timeout in seconds (max 900) |
| `log_level` | `info` | Gateway log level (debug, info, warn, error) |
| `enable_authorization` | `false` | Enable JWT authorization |
| `enable_rate_limiting` | `true` | Enable rate limiting |
| `dynamodb_billing_mode` | `PAY_PER_REQUEST` | DynamoDB billing mode |

### Custom Domain Configuration

To use a custom domain (e.g., `api.example.com`):

1. Ensure you have a Route53 hosted zone
2. Update `terraform.tfvars`:

```hcl
enable_custom_domain = true
domain_name          = "api.example.com"
route53_zone_id      = "Z1234567890ABC"
```

3. Apply changes:

```bash
terraform apply
```

Certificate validation may take 5-10 minutes.

## Backend Configuration (Remote State)

For team collaboration, use remote state storage:

### S3 Backend

1. Create S3 bucket and DynamoDB table:

```bash
aws s3 mb s3://my-terraform-state-bucket --region us-east-1
aws dynamodb create-table \
  --table-name terraform-state-lock \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

2. Uncomment backend configuration in `main.tf`:

```hcl
backend "s3" {
  bucket         = "my-terraform-state-bucket"
  key            = "api-gateway/terraform.tfstate"
  region         = "us-east-1"
  dynamodb_table = "terraform-state-lock"
  encrypt        = true
}
```

3. Re-initialize Terraform:

```bash
terraform init -migrate-state
```

## Updating the Deployment

### Updating Lambda Code

1. Build and push new Docker image to ECR
2. Update the Lambda function:

```bash
# AWS CLI method (no Terraform needed)
aws lambda update-function-code \
  --function-name api-gateway-dev-gateway \
  --image-uri <account-id>.dkr.ecr.us-east-1.amazonaws.com/api-gateway:latest
```

### Updating Infrastructure

```bash
# Make changes to .tf files or terraform.tfvars
terraform plan   # Review changes
terraform apply  # Apply changes
```

## Monitoring and Logs

### CloudWatch Logs

```bash
# View Lambda logs
aws logs tail /aws/lambda/api-gateway-dev-gateway --follow

# View API Gateway logs
aws logs tail /aws/apigateway/api-gateway-dev-api --follow
```

### Metrics

- **Lambda Metrics**: CloudWatch → Lambda → Functions → api-gateway-dev-gateway
- **API Gateway Metrics**: CloudWatch → API Gateway → api-gateway-dev-api
- **DynamoDB Metrics**: CloudWatch → DynamoDB → Tables

### CloudWatch Insights Queries

```sql
# Find slow requests
fields @timestamp, @message
| filter @message like /duration_ms/
| parse @message /"duration_ms":(?<duration>\d+)/
| filter duration > 1000
| sort duration desc
| limit 20

# Error rate
fields @timestamp, @message
| filter @message like /ERROR/
| stats count() by bin(5m)
```

## Troubleshooting

### Lambda Cold Starts

**Problem:** First request after idle period is slow (~1-2 seconds)

**Solutions:**
- Increase memory (more CPU allocation)
- Enable provisioned concurrency (costs more)
- Use CloudWatch Events to keep warm

### Rate Limiting Not Working

**Problem:** Rate limiting not enforced

**Checks:**
1. Verify DynamoDB table exists: `aws dynamodb list-tables`
2. Check IAM permissions for Lambda to access DynamoDB
3. Enable rate limiting: `enable_rate_limiting = true` in terraform.tfvars
4. Check CloudWatch logs for rate limit errors

### Backend Connection Errors

**Problem:** Lambda can't reach backend services

**Solutions:**
- Ensure backend URLs are correct and reachable
- Check VPC configuration if backends are in VPC
- Verify security groups allow outbound HTTPS
- Check Lambda execution role has network access

### Terraform State Locked

**Problem:** `Error: Error locking state`

**Solution:**
```bash
# Unlock state (use with caution)
terraform force-unlock <lock-id>
```

## Cost Optimization Tips

1. **Use On-Demand Pricing**: DynamoDB PAY_PER_REQUEST is cheaper for low/variable traffic
2. **Reduce Log Retention**: Set to 7 days for dev, 30 days for prod
3. **Right-Size Lambda**: Start with 512MB memory, adjust based on metrics
4. **Use HTTP API**: Cheaper than REST API (~70% cost reduction)
5. **Monitor Unused Resources**: Regularly check AWS Cost Explorer
6. **Set Budget Alerts**: Configure AWS Budgets to alert on costs

## Security Best Practices

1. **Never commit secrets** to version control
2. **Use AWS Secrets Manager** for JWT secrets in production
3. **Enable CloudTrail** for audit logging
4. **Restrict IAM policies** to least privilege
5. **Enable DynamoDB encryption** at rest (default enabled)
6. **Use HTTPS only** (enforced by API Gateway)
7. **Enable VPC endpoints** if accessing VPC resources
8. **Review security groups** regularly

## Cleanup

To destroy all resources:

```bash
terraform destroy
```

**⚠️ WARNING:** This permanently deletes:
- Lambda function
- API Gateway
- DynamoDB table (and all data)
- CloudWatch Logs
- IAM roles
- Custom domain (if configured)

Estimated savings: All AWS costs eliminated.

## Advanced Configuration

### Multi-Environment Deployment

Use Terraform workspaces or separate state files:

```bash
# Using workspaces
terraform workspace new staging
terraform workspace select staging
terraform apply -var="environment=staging"

# Using separate directories
cp -r terraform terraform-prod
cd terraform-prod
# Update terraform.tfvars with prod settings
terraform apply
```

### VPC Configuration

If backend services are in a VPC, enable VPC networking:

1. Uncomment VPC configuration in `modules/lambda/main.tf`
2. Add VPC variables to `variables.tf`
3. Configure security groups and subnets

### Lambda Layers

To reduce deployment size, use Lambda layers for dependencies:

```hcl
resource "aws_lambda_layer_version" "gateway_deps" {
  filename   = "lambda-layer.zip"
  layer_name = "gateway-dependencies"
  compatible_runtimes = ["provided.al2023"]
}
```

## Support and Contributing

For issues or questions:
- Review [AWS Lambda Documentation](https://docs.aws.amazon.com/lambda/)
- Check [Terraform AWS Provider Docs](https://registry.terraform.io/providers/hashicorp/aws/)
- See main project [README](../README.md)

## License

Same as main project.
