# ========================================
# Local Variables
# ========================================

locals {
  name_prefix = "${var.project_name}-${var.environment}"

  # Lambda function definitions
  lambda_functions = {
    user-service = {
      handler     = "bootstrap"
      runtime     = "provided.al2023" # For Go custom runtime
      memory_size = var.lambda_memory_size
      timeout     = var.lambda_timeout
      environment_variables = merge(
        var.lambda_environment_variables,
        {
          ENVIRONMENT = var.environment
          LOG_LEVEL   = var.environment == "prod" ? "warn" : "info"
        }
      )
    }
    order-service = {
      handler     = "bootstrap"
      runtime     = "provided.al2023"
      memory_size = var.lambda_memory_size
      timeout     = var.lambda_timeout
      environment_variables = merge(
        var.lambda_environment_variables,
        {
          ENVIRONMENT = var.environment
          LOG_LEVEL   = var.environment == "prod" ? "warn" : "info"
        }
      )
    }
    admin-service = {
      handler     = "bootstrap"
      runtime     = "provided.al2023"
      memory_size = var.lambda_memory_size
      timeout     = var.lambda_timeout
      environment_variables = merge(
        var.lambda_environment_variables,
        {
          ENVIRONMENT = var.environment
          LOG_LEVEL   = var.environment == "prod" ? "warn" : "info"
        }
      )
    }
    status-service = {
      handler     = "bootstrap"
      runtime     = "provided.al2023"
      memory_size = 128 # Status service needs less memory
      timeout     = 5
      environment_variables = {
        ENVIRONMENT = var.environment
      }
    }
  }

  # API routes configuration
  api_routes = {
    # User service routes
    "GET /api/v1/users" = {
      function_name      = "user-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }
    "POST /api/v1/users" = {
      function_name      = "user-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }
    "GET /api/v1/users/{id}" = {
      function_name      = "user-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }
    "PUT /api/v1/users/{id}" = {
      function_name      = "user-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }
    "DELETE /api/v1/users/{id}" = {
      function_name      = "user-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }

    # Order service routes
    "GET /api/v1/orders" = {
      function_name      = "order-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }
    "POST /api/v1/orders" = {
      function_name      = "order-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }
    "GET /api/v1/orders/{id}" = {
      function_name      = "order-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }
    "PUT /api/v1/orders/{id}" = {
      function_name      = "order-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }
    "DELETE /api/v1/orders/{id}" = {
      function_name      = "order-service"
      authorization_type = "JWT"
      throttle_settings  = null
    }

    # Admin service routes (requires JWT)
    "GET /api/v1/admin" = {
      function_name      = "admin-service"
      authorization_type = "JWT"
      throttle_settings = {
        rate_limit  = 20  # Lower limit for admin routes
        burst_limit = 10
      }
    }
    "POST /api/v1/admin" = {
      function_name      = "admin-service"
      authorization_type = "JWT"
      throttle_settings = {
        rate_limit  = 20
        burst_limit = 10
      }
    }

    # Public health check route (no auth)
    "GET /api/v1/public/health" = {
      function_name      = "status-service"
      authorization_type = "NONE"
      throttle_settings  = null
    }

    # System health check routes (no auth)
    "GET /_health" = {
      function_name      = "status-service"
      authorization_type = "NONE"
      throttle_settings  = null
    }
    "GET /_health/ready" = {
      function_name      = "status-service"
      authorization_type = "NONE"
      throttle_settings  = null
    }
    "GET /_health/live" = {
      function_name      = "status-service"
      authorization_type = "NONE"
      throttle_settings  = null
    }
  }
}

# ========================================
# Lambda Functions
# ========================================

module "lambda_functions" {
  source = "./modules/lambda"

  for_each = local.lambda_functions

  function_name = "${local.name_prefix}-${each.key}"
  description   = "${each.key} Lambda function for ${var.environment} environment"

  handler       = each.value.handler
  runtime       = each.value.runtime
  memory_size   = each.value.memory_size
  timeout       = each.value.timeout

  source_dir    = "${path.module}/lambda-src/${each.key}"

  environment_variables = each.value.environment_variables

  log_retention_days = var.lambda_log_retention_days

  reserved_concurrent_executions = var.lambda_reserved_concurrent_executions

  # Grant Lambda access to DynamoDB tables
  dynamodb_table_arns = var.enable_dynamodb ? [
    for table_name, _ in var.dynamodb_tables :
    module.dynamodb_tables[table_name].table_arn
  ] : []

  tags = merge(
    var.common_tags,
    {
      Environment = var.environment
      Function    = each.key
    }
  )
}

# ========================================
# DynamoDB Tables
# ========================================

module "dynamodb_tables" {
  source = "./modules/dynamodb"

  for_each = var.enable_dynamodb ? var.dynamodb_tables : {}

  table_name    = "${local.name_prefix}-${each.key}"
  billing_mode  = var.dynamodb_billing_mode

  hash_key      = each.value.hash_key
  range_key     = each.value.range_key

  attributes    = each.value.attributes

  global_secondary_indexes = each.value.global_secondary_indexes

  stream_enabled   = each.value.stream_enabled
  stream_view_type = each.value.stream_view_type

  point_in_time_recovery = var.dynamodb_point_in_time_recovery
  deletion_protection    = var.dynamodb_deletion_protection

  tags = merge(
    var.common_tags,
    {
      Environment = var.environment
      Table       = each.key
    }
  )
}

# ========================================
# API Gateway HTTP API
# ========================================

module "api_gateway" {
  source = "./modules/api-gateway"

  name        = local.name_prefix
  description = "API Gateway for ${var.environment} environment"

  # Throttling configuration
  throttle_burst_limit = var.api_gateway_throttle_burst_limit
  throttle_rate_limit  = var.api_gateway_throttle_rate_limit

  # JWT Authorizer
  enable_jwt_authorizer = var.enable_jwt_authorizer
  jwt_issuer           = var.jwt_issuer
  jwt_audience         = var.jwt_audience
  jwt_identity_source  = var.jwt_identity_source

  # CORS configuration
  enable_cors        = var.enable_cors
  cors_allow_origins = var.cors_allow_origins
  cors_allow_methods = var.cors_allow_methods
  cors_allow_headers = var.cors_allow_headers
  cors_max_age       = var.cors_max_age

  # Routes configuration
  routes = {
    for route_key, route_config in local.api_routes : route_key => {
      lambda_invoke_arn  = module.lambda_functions[route_config.function_name].invoke_arn
      authorization_type = var.enable_jwt_authorizer && route_config.authorization_type == "JWT" ? "JWT" : "NONE"
      throttle_settings  = route_config.throttle_settings
    }
  }

  # Lambda permissions
  lambda_permissions = {
    for route_key, route_config in local.api_routes : route_key => {
      function_name = module.lambda_functions[route_config.function_name].function_name
    }
  }

  # Logging
  enable_access_logs = var.enable_api_gateway_logs
  log_retention_days = var.api_gateway_log_retention_days

  tags = merge(
    var.common_tags,
    {
      Environment = var.environment
    }
  )
}

# ========================================
# Custom Domain (Optional)
# ========================================

module "custom_domain" {
  source = "./modules/custom-domain"

  count = var.enable_custom_domain ? 1 : 0

  domain_name     = var.domain_name
  certificate_arn = var.certificate_arn

  api_gateway_id           = module.api_gateway.api_id
  api_gateway_stage_name   = module.api_gateway.stage_name
  api_gateway_stage_invoke_url = module.api_gateway.api_endpoint

  route53_zone_id = var.route53_zone_id

  tags = merge(
    var.common_tags,
    {
      Environment = var.environment
    }
  )
}
