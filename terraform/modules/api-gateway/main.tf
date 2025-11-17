# ========================================
# API Gateway HTTP API
# ========================================

resource "aws_apigatewayv2_api" "this" {
  name          = var.name
  description   = var.description
  protocol_type = "HTTP"

  cors_configuration {
    allow_origins     = var.enable_cors ? var.cors_allow_origins : []
    allow_methods     = var.enable_cors ? var.cors_allow_methods : []
    allow_headers     = var.enable_cors ? var.cors_allow_headers : []
    max_age           = var.enable_cors ? var.cors_max_age : 0
    expose_headers    = ["X-Request-Id", "X-RateLimit-Limit", "X-RateLimit-Remaining"]
    allow_credentials = false
  }

  tags = var.tags
}

# ========================================
# JWT Authorizer
# ========================================

resource "aws_apigatewayv2_authorizer" "jwt" {
  count = var.enable_jwt_authorizer ? 1 : 0

  api_id           = aws_apigatewayv2_api.this.id
  authorizer_type  = "JWT"
  identity_sources = [var.jwt_identity_source]
  name             = "${var.name}-jwt-authorizer"

  jwt_configuration {
    audience = var.jwt_audience
    issuer   = var.jwt_issuer
  }
}

# ========================================
# API Gateway Stage
# ========================================

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.this.id
  name        = "$default"
  auto_deploy = true

  default_route_settings {
    throttle_burst_limit = var.throttle_burst_limit
    throttle_rate_limit  = var.throttle_rate_limit

    # Enable detailed metrics
    detailed_metrics_enabled = true
  }

  access_log_settings {
    destination_arn = var.enable_access_logs ? aws_cloudwatch_log_group.api_gateway[0].arn : null
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      caller         = "$context.identity.caller"
      user           = "$context.identity.user"
      requestTime    = "$context.requestTime"
      httpMethod     = "$context.httpMethod"
      resourcePath   = "$context.resourcePath"
      status         = "$context.status"
      protocol       = "$context.protocol"
      responseLength = "$context.responseLength"
      errorMessage   = "$context.error.message"
      integrationErrorMessage = "$context.integrationErrorMessage"
      authorizerError = "$context.authorizer.error"
    })
  }

  tags = var.tags
}

# ========================================
# CloudWatch Log Group for API Gateway
# ========================================

resource "aws_cloudwatch_log_group" "api_gateway" {
  count = var.enable_access_logs ? 1 : 0

  name              = "/aws/apigateway/${var.name}"
  retention_in_days = var.log_retention_days

  tags = var.tags
}

# ========================================
# API Gateway Routes and Integrations
# ========================================

resource "aws_apigatewayv2_integration" "lambda" {
  for_each = var.routes

  api_id           = aws_apigatewayv2_api.this.id
  integration_type = "AWS_PROXY"

  connection_type      = "INTERNET"
  integration_method   = "POST"
  integration_uri      = each.value.lambda_invoke_arn
  payload_format_version = "2.0"

  # Timeout configuration (max 30s for HTTP APIs)
  timeout_milliseconds = 29000
}

resource "aws_apigatewayv2_route" "this" {
  for_each = var.routes

  api_id    = aws_apigatewayv2_api.this.id
  route_key = each.key

  target = "integrations/${aws_apigatewayv2_integration.lambda[each.key].id}"

  # Authorization
  authorization_type = each.value.authorization_type
  authorizer_id      = each.value.authorization_type == "JWT" && var.enable_jwt_authorizer ? aws_apigatewayv2_authorizer.jwt[0].id : null

  # Per-route throttling (if specified)
  dynamic "throttle_settings" {
    for_each = each.value.throttle_settings != null ? [each.value.throttle_settings] : []
    content {
      burst_limit = throttle_settings.value.burst_limit
      rate_limit  = throttle_settings.value.rate_limit
    }
  }
}

# ========================================
# Lambda Permissions for API Gateway
# ========================================

resource "aws_lambda_permission" "api_gateway" {
  for_each = var.lambda_permissions

  statement_id  = "AllowExecutionFromAPIGateway-${replace(each.key, "/[^a-zA-Z0-9]/", "")}"
  action        = "lambda:InvokeFunction"
  function_name = each.value.function_name
  principal     = "apigateway.amazonaws.com"

  source_arn = "${aws_apigatewayv2_api.this.execution_arn}/*/*"
}
