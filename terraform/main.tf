# Main Terraform Configuration
# Deploys API Gateway on AWS Lambda with serverless architecture

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Uncomment and configure for remote state
  # backend "s3" {
  #   bucket         = "my-terraform-state-bucket"
  #   key            = "api-gateway/terraform.tfstate"
  #   region         = "us-east-1"
  #   dynamodb_table = "terraform-state-lock"
  #   encrypt        = true
  # }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = merge(
      {
        Project     = var.project_name
        Environment = var.environment
        ManagedBy   = "Terraform"
      },
      var.tags
    )
  }
}

# Local values for resource naming
locals {
  name_prefix = "${var.project_name}-${var.environment}"
  common_tags = {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "Terraform"
  }
}

# DynamoDB Table for Rate Limiting
module "dynamodb" {
  source = "./modules/dynamodb"

  table_name     = "${local.name_prefix}-rate-limits"
  billing_mode   = var.dynamodb_billing_mode
  read_capacity  = var.dynamodb_read_capacity
  write_capacity = var.dynamodb_write_capacity
  tags           = local.common_tags
}

# IAM Roles and Policies
module "iam" {
  source = "./modules/iam"

  name_prefix        = local.name_prefix
  dynamodb_table_arn = module.dynamodb.table_arn
  tags               = local.common_tags
}

# Lambda Function
module "lambda" {
  source = "./modules/lambda"

  function_name      = "${local.name_prefix}-gateway"
  image_uri          = var.lambda_image_uri
  execution_role_arn = module.iam.lambda_role_arn
  memory_size        = var.lambda_memory_size
  timeout            = var.lambda_timeout
  log_retention_days = var.lambda_log_retention_days

  environment_variables = {
    GATEWAY_HTTP_PORT          = "8080"
    GATEWAY_LOG_LEVEL          = var.log_level
    GATEWAY_LOG_FORMAT         = "json"
    GATEWAY_AUTH_ENABLED       = tostring(var.enable_authorization)
    GATEWAY_RATELIMIT_ENABLED  = tostring(var.enable_rate_limiting)
    GATEWAY_RATELIMIT_BACKEND  = "dynamodb"
    GATEWAY_DYNAMODB_TABLE     = module.dynamodb.table_name
    GATEWAY_DYNAMODB_REGION    = var.aws_region
    GATEWAY_BACKEND_USERS_URL  = var.backend_users_url
    GATEWAY_BACKEND_ORDERS_URL = var.backend_orders_url
    GATEWAY_JWT_SHARED_SECRET  = var.jwt_shared_secret
    AWS_LWA_ENABLE_COMPRESSION = "true"
    AWS_LWA_INVOKE_MODE        = "response_stream"
  }

  tags = local.common_tags
}

# API Gateway HTTP API
module "api_gateway" {
  source = "./modules/api-gateway"

  api_name            = "${local.name_prefix}-api"
  lambda_function_arn = module.lambda.function_arn
  lambda_invoke_arn   = module.lambda.invoke_arn
  tags                = local.common_tags
}

# Custom Domain (Optional)
module "custom_domain" {
  source = "./modules/custom-domain"
  count  = var.enable_custom_domain ? 1 : 0

  domain_name     = var.domain_name
  api_id          = module.api_gateway.api_id
  api_stage_name  = module.api_gateway.stage_name
  route53_zone_id = var.route53_zone_id
  tags            = local.common_tags
}
