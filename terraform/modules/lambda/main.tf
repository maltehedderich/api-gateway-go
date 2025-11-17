# Lambda Module
# Creates Lambda function from container image

# Lambda function
resource "aws_lambda_function" "gateway" {
  function_name = var.function_name
  role          = var.execution_role_arn
  package_type  = "Image"
  image_uri     = var.image_uri
  memory_size   = var.memory_size
  timeout       = var.timeout

  environment {
    variables = var.environment_variables
  }

  # Reserved concurrent executions (set to limit max instances)
  # Reserved concurrency of 0 prevents function from running
  # Leave null for unreserved (default)
  reserved_concurrent_executions = var.reserved_concurrent_executions

  tags = var.tags

  # Lifecycle rule to prevent changes from external updates
  lifecycle {
    ignore_changes = [
      image_uri, # Ignore if image is updated outside Terraform
    ]
  }
}

# CloudWatch Log Group for Lambda
resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = var.log_retention_days
  tags              = var.tags
}

# Lambda Function URL (alternative to API Gateway for simpler deployments)
# Uncomment to enable direct Lambda Function URL
# resource "aws_lambda_function_url" "gateway" {
#   function_name      = aws_lambda_function.gateway.function_name
#   authorization_type = "NONE"
#
#   cors {
#     allow_origins     = ["*"]
#     allow_methods     = ["*"]
#     allow_headers     = ["*"]
#     expose_headers    = ["*"]
#     max_age           = 86400
#   }
# }
