# API Gateway - Deployment Guide

This guide walks you through deploying the API Gateway to AWS from scratch.

## Prerequisites Checklist

Before starting, ensure you have:

- [ ] AWS account with appropriate permissions
- [ ] AWS CLI installed and configured
- [ ] Terraform >= 1.5.0 installed
- [ ] Go >= 1.21 installed
- [ ] Make installed (optional, for convenience)

## Step-by-Step Deployment

### Step 1: Clone Repository and Navigate

```bash
cd api-gateway-go/terraform
```

### Step 2: Configure AWS Credentials

**Option A: AWS CLI Configuration**

```bash
aws configure
```

Enter your credentials:
- AWS Access Key ID
- AWS Secret Access Key
- Default region (e.g., `us-east-1`)
- Default output format (e.g., `json`)

**Option B: Environment Variables**

```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_DEFAULT_REGION="us-east-1"
```

**Option C: AWS SSO**

```bash
aws sso login --profile my-profile
export AWS_PROFILE=my-profile
```

**Verify credentials:**

```bash
aws sts get-caller-identity
```

### Step 3: Build Lambda Functions

Build the Lambda function binaries:

```bash
./scripts/build-lambdas.sh
```

Expected output:
```
📦 Building user-service...
  → Running go mod download...
  → Building binary for Linux/amd64...
✓ Built user-service successfully (8.1M)

📦 Building order-service...
  → Running go mod download...
  → Building binary for Linux/amd64...
✓ Built order-service successfully (8.2M)
...
```

**Or using Make:**

```bash
make build
```

### Step 4: Configure Terraform Variables

Copy the example configuration:

```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your settings:

```bash
nano terraform.tfvars  # or vim, code, etc.
```

**Minimum required configuration:**

```hcl
aws_region   = "us-east-1"
environment  = "dev"
project_name = "api-gateway"

# JWT Configuration (IMPORTANT!)
enable_jwt_authorizer = true
jwt_issuer           = "https://your-auth-service.com"  # ← CHANGE THIS
jwt_audience         = ["api-gateway"]
```

**For development without JWT authentication:**

```hcl
enable_jwt_authorizer = false
```

### Step 5: Initialize Terraform

Initialize Terraform and download providers:

```bash
terraform init
```

Expected output:
```
Initializing modules...
Initializing the backend...
Initializing provider plugins...
...
Terraform has been successfully initialized!
```

**Or using Make:**

```bash
make init
```

### Step 6: Validate Configuration

Validate your Terraform configuration:

```bash
terraform validate
```

Expected output:
```
Success! The configuration is valid.
```

**Check formatting:**

```bash
terraform fmt -check -recursive
```

### Step 7: Plan Deployment

Preview the infrastructure changes:

```bash
terraform plan
```

Review the plan output. You should see resources like:
- API Gateway HTTP API
- 4 Lambda functions
- 2 DynamoDB tables
- CloudWatch Log Groups
- IAM roles and policies

**Or using Make:**

```bash
make plan
```

**Save the plan for later apply:**

```bash
terraform plan -out=tfplan
```

### Step 8: Deploy Infrastructure

Apply the Terraform configuration:

```bash
terraform apply
```

Review the plan and type `yes` to confirm.

**Or using saved plan:**

```bash
terraform apply tfplan
```

**Or using Make (with auto-approve, use with caution):**

```bash
make apply-auto
```

Deployment takes approximately **2-3 minutes**.

### Step 9: Verify Deployment

After deployment completes, Terraform outputs the API endpoint:

```bash
terraform output api_endpoint
```

Example output:
```
https://abc123def4.execute-api.us-east-1.amazonaws.com
```

**View all outputs:**

```bash
terraform output
```

**Or using Make:**

```bash
make output
```

### Step 10: Test API

**Test health endpoint (no authentication required):**

```bash
API_ENDPOINT=$(terraform output -raw api_endpoint)
curl $API_ENDPOINT/_health
```

Expected response:
```json
{
  "status": "healthy",
  "environment": "dev",
  "timestamp": "2025-11-17T10:30:00Z",
  "service": "status-service"
}
```

**Or using Make:**

```bash
make test-health
```

**Test authenticated endpoint (JWT required):**

```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     $API_ENDPOINT/api/v1/users
```

Expected response (with valid JWT):
```json
{
  "users": [],
  "count": 0
}
```

**Or using Make:**

```bash
make test-auth JWT_TOKEN=your_jwt_token_here
```

---

## Post-Deployment Tasks

### 1. Configure JWT Authentication

If using JWT authentication, you need to set up your JWT issuer:

1. **Set up Auth0, Cognito, or your own auth service**
2. **Configure JWKS endpoint** at `https://your-issuer/.well-known/jwks.json`
3. **Update `jwt_issuer` in `terraform.tfvars`** to match your issuer URL
4. **Generate test JWT tokens** with required claims:
   - `iss`: Must match `jwt_issuer`
   - `aud`: Must be in `jwt_audience`
   - `sub`: User ID
   - `exp`: Expiration timestamp

### 2. Set Up Custom Domain (Optional)

If you want to use a custom domain like `api.example.com`:

**Step 1: Request ACM Certificate**

```bash
aws acm request-certificate \
  --domain-name api.example.com \
  --validation-method DNS \
  --region us-east-1
```

**Step 2: Validate Certificate**

Add the DNS validation CNAME record to your domain's DNS.

**Step 3: Get Certificate ARN**

```bash
aws acm list-certificates --region us-east-1
```

**Step 4: Update `terraform.tfvars`**

```hcl
enable_custom_domain = true
domain_name         = "api.example.com"
certificate_arn     = "arn:aws:acm:us-east-1:123456789012:certificate/..."
route53_zone_id     = "Z1234567890ABC"
```

**Step 5: Apply Changes**

```bash
terraform apply
```

Your API will be accessible at `https://api.example.com`.

### 3. Configure CloudWatch Alarms

Set up alarms for monitoring:

```bash
# API Gateway 5xx errors
aws cloudwatch put-metric-alarm \
  --alarm-name api-gateway-5xx-errors \
  --alarm-description "Alert on API Gateway 5xx errors" \
  --metric-name 5XXError \
  --namespace AWS/ApiGateway \
  --statistic Sum \
  --period 300 \
  --evaluation-periods 1 \
  --threshold 10 \
  --comparison-operator GreaterThanThreshold

# Lambda errors
aws cloudwatch put-metric-alarm \
  --alarm-name lambda-errors \
  --alarm-description "Alert on Lambda errors" \
  --metric-name Errors \
  --namespace AWS/Lambda \
  --statistic Sum \
  --period 300 \
  --evaluation-periods 1 \
  --threshold 5 \
  --comparison-operator GreaterThanThreshold
```

### 4. Set Up Cost Alerts

Create a budget to monitor costs:

```bash
aws budgets create-budget \
  --account-id $(aws sts get-caller-identity --query Account --output text) \
  --budget file://budget.json
```

**budget.json:**
```json
{
  "BudgetName": "API Gateway Monthly Budget",
  "BudgetLimit": {
    "Amount": "10",
    "Unit": "USD"
  },
  "BudgetType": "COST",
  "TimeUnit": "MONTHLY"
}
```

### 5. Enable AWS X-Ray Tracing

X-Ray tracing is already enabled for Lambda functions. View traces:

1. Open AWS X-Ray console
2. Navigate to Service Map
3. View traces for your Lambda functions

---

## Monitoring and Logs

### View CloudWatch Logs

**API Gateway Logs:**

```bash
aws logs tail /aws/apigateway/api-gateway-dev --follow
```

**Lambda Logs:**

```bash
aws logs tail /aws/lambda/api-gateway-dev-user-service --follow
```

**Or using Make:**

```bash
make logs-api
make logs-lambda SERVICE=user-service
```

### View Metrics

Open CloudWatch console and navigate to:
- **API Gateway Metrics**: Request count, latency, errors
- **Lambda Metrics**: Invocations, duration, errors, throttles
- **DynamoDB Metrics**: Consumed capacity, throttled requests

---

## Updating the Deployment

### Update Lambda Function Code

1. **Modify Lambda source code** in `lambda-src/`
2. **Rebuild functions:**
   ```bash
   ./scripts/build-lambdas.sh
   ```
3. **Apply changes:**
   ```bash
   terraform apply
   ```

### Update Configuration

1. **Edit `terraform.tfvars`**
2. **Preview changes:**
   ```bash
   terraform plan
   ```
3. **Apply changes:**
   ```bash
   terraform apply
   ```

### Add New Lambda Function

1. **Create directory:** `lambda-src/my-service/`
2. **Add Go code:** `lambda-src/my-service/main.go`
3. **Add go.mod:** `lambda-src/my-service/go.mod`
4. **Update `main.tf`**: Add to `lambda_functions` local variable
5. **Add routes**: Update `api_routes` local variable
6. **Build and deploy:**
   ```bash
   ./scripts/build-lambdas.sh
   terraform apply
   ```

---

## Multi-Environment Deployment

### Option 1: Terraform Workspaces

```bash
# Create staging workspace
terraform workspace new staging

# Switch to staging
terraform workspace select staging

# Deploy to staging
terraform apply -var="environment=staging"

# Switch back to dev
terraform workspace select default
```

### Option 2: Separate Directories

```bash
# Copy terraform directory
cp -r terraform terraform-staging

# Navigate to staging
cd terraform-staging

# Update terraform.tfvars with staging config
nano terraform.tfvars

# Initialize and deploy
terraform init
terraform apply
```

### Option 3: Separate tfvars Files

```bash
# Create environment-specific tfvars
cp terraform.tfvars terraform.dev.tfvars
cp terraform.tfvars terraform.staging.tfvars

# Deploy to specific environment
terraform apply -var-file=terraform.staging.tfvars
```

---

## Rollback Procedure

If deployment fails or you need to rollback:

### Option 1: Rollback to Previous State

```bash
# View state history
terraform state list

# Show previous state
terraform state show aws_lambda_function.user

# Import previous version (if needed)
terraform import aws_lambda_function.user previous-function-name
```

### Option 2: Destroy and Redeploy

```bash
# Destroy current deployment
terraform destroy

# Checkout previous working commit
git checkout <previous-commit>

# Redeploy
make full-deploy
```

### Option 3: Manual Rollback in AWS Console

1. Open Lambda console
2. Navigate to function → Versions
3. Promote previous version to $LATEST
4. Refresh Terraform state:
   ```bash
   terraform refresh
   ```

---

## Disaster Recovery

### Backup DynamoDB Tables

**Enable Point-in-Time Recovery:**

```hcl
dynamodb_point_in_time_recovery = true
```

**Manual Backup:**

```bash
aws dynamodb create-backup \
  --table-name api-gateway-dev-users \
  --backup-name users-backup-$(date +%Y%m%d)
```

**Restore from Backup:**

```bash
aws dynamodb restore-table-from-backup \
  --target-table-name api-gateway-dev-users-restored \
  --backup-arn arn:aws:dynamodb:us-east-1:123456789012:table/api-gateway-dev-users/backup/...
```

### Backup Terraform State

**If using local state:**

```bash
cp terraform.tfstate terraform.tfstate.backup
```

**If using S3 backend:**

```bash
aws s3 cp s3://your-terraform-state-bucket/api-gateway/terraform.tfstate \
          terraform.tfstate.backup
```

---

## Troubleshooting

### Issue: Lambda Build Fails

**Error:**
```
go: command not found
```

**Solution:**
Install Go from https://go.dev/dl/

---

### Issue: JWT Authorization Fails

**Error:**
```
{"message":"Unauthorized"}
```

**Solutions:**

1. **Check JWT issuer matches:**
   ```bash
   echo "Configured: $(terraform output -raw jwt_issuer)"
   echo "Token issuer: <decode JWT and check 'iss' claim>"
   ```

2. **Verify JWT audience:**
   - JWT `aud` claim must be in `jwt_audience` list

3. **Check token expiration:**
   - JWT `exp` claim must be in the future

4. **View CloudWatch logs:**
   ```bash
   make logs-api
   ```

---

### Issue: DynamoDB Access Denied

**Error:**
```
AccessDeniedException
```

**Solution:**
Check IAM role permissions. Terraform automatically grants permissions, so this usually means:
1. Role policy not yet propagated (wait 30 seconds)
2. Table name mismatch in environment variables

---

### Issue: Terraform State Lock

**Error:**
```
Error acquiring the state lock
```

**Solution:**
```bash
# View lock info
terraform force-unlock <LOCK_ID>

# Or if using DynamoDB for locks
aws dynamodb delete-item \
  --table-name terraform-state-locks \
  --key '{"LockID": {"S": "..."}'
```

---

## Cleanup

To destroy all resources and clean up:

```bash
# Preview what will be destroyed
terraform plan -destroy

# Destroy all resources
terraform destroy
```

**⚠️ WARNING**: This will delete:
- All DynamoDB tables and data
- All Lambda functions
- API Gateway
- CloudWatch logs

**Backup data before destroying!**

**Or using Make:**

```bash
make destroy
```

---

## Next Steps

After successful deployment:

1. ✅ Test all API endpoints
2. ✅ Set up monitoring and alarms
3. ✅ Configure custom domain (optional)
4. ✅ Enable point-in-time recovery for DynamoDB (production)
5. ✅ Set up CI/CD pipeline
6. ✅ Document API endpoints for your team
7. ✅ Load test the API
8. ✅ Configure AWS WAF for security (production)

---

## Support

For issues and questions:

1. Check CloudWatch logs
2. Review [README.md](README.md) troubleshooting section
3. Open an issue in the repository

---

## Deployment Checklist

Use this checklist for production deployments:

- [ ] AWS credentials configured
- [ ] Terraform initialized
- [ ] Lambda functions built successfully
- [ ] `terraform.tfvars` configured correctly
- [ ] JWT issuer configured and reachable
- [ ] ACM certificate validated (if using custom domain)
- [ ] `terraform plan` reviewed
- [ ] Deployment applied successfully
- [ ] Health endpoint responding
- [ ] Authenticated endpoints tested
- [ ] CloudWatch logs flowing
- [ ] Metrics visible in CloudWatch
- [ ] Cost alerts configured
- [ ] Monitoring alarms set up
- [ ] DynamoDB backups enabled (production)
- [ ] Documentation updated
- [ ] Team notified of deployment

---

## Production Readiness Checklist

Before going to production:

- [ ] Enable deletion protection for DynamoDB
- [ ] Enable point-in-time recovery for DynamoDB
- [ ] Configure custom domain with ACM certificate
- [ ] Restrict CORS origins to specific domains
- [ ] Increase log retention (30+ days)
- [ ] Set up CloudWatch alarms
- [ ] Enable AWS X-Ray tracing
- [ ] Configure AWS WAF for API Gateway
- [ ] Set up automated backups
- [ ] Document disaster recovery procedures
- [ ] Load test the API
- [ ] Security review completed
- [ ] Compliance requirements met
- [ ] Monitoring dashboards created
- [ ] On-call rotation established
