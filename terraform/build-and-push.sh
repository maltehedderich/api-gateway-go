#!/bin/bash
# Build and push Docker image to ECR for Lambda deployment

set -e

# Configuration
AWS_REGION="${AWS_REGION:-us-east-1}"
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REPOSITORY="${ECR_REPOSITORY:-api-gateway}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

# Full image name
IMAGE_URI="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPOSITORY}:${IMAGE_TAG}"

echo "==> Building Docker image for Lambda..."
echo "    Image URI: ${IMAGE_URI}"

# Navigate to project root (script is in terraform/ directory)
cd "$(dirname "$0")/.."

# Build Docker image
docker build \
  --platform linux/amd64 \
  -f Dockerfile.lambda \
  -t "${ECR_REPOSITORY}:${IMAGE_TAG}" \
  .

echo "==> Docker image built successfully"

# Tag for ECR
docker tag "${ECR_REPOSITORY}:${IMAGE_TAG}" "${IMAGE_URI}"

# Login to ECR
echo "==> Logging in to ECR..."
aws ecr get-login-password --region "${AWS_REGION}" | \
  docker login --username AWS --password-stdin \
  "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

# Create ECR repository if it doesn't exist
echo "==> Ensuring ECR repository exists..."
aws ecr describe-repositories \
  --repository-names "${ECR_REPOSITORY}" \
  --region "${AWS_REGION}" &>/dev/null || \
aws ecr create-repository \
  --repository-name "${ECR_REPOSITORY}" \
  --region "${AWS_REGION}" \
  --image-scanning-configuration scanOnPush=true \
  --encryption-configuration encryptionType=AES256

# Push image to ECR
echo "==> Pushing image to ECR..."
docker push "${IMAGE_URI}"

echo "==> Image pushed successfully!"
echo ""
echo "Next steps:"
echo "  1. Update terraform/terraform.tfvars with:"
echo "     lambda_image_uri = \"${IMAGE_URI}\""
echo "  2. Run: terraform apply"
echo ""
