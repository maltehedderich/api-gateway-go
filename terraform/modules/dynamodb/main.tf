# DynamoDB Module
# Creates a DynamoDB table for rate limiting state

resource "aws_dynamodb_table" "rate_limits" {
  name         = var.table_name
  billing_mode = var.billing_mode
  hash_key     = "key"
  range_key    = "window"

  # Only set capacity if using PROVISIONED billing mode
  read_capacity  = var.billing_mode == "PROVISIONED" ? var.read_capacity : null
  write_capacity = var.billing_mode == "PROVISIONED" ? var.write_capacity : null

  attribute {
    name = "key"
    type = "S"
  }

  attribute {
    name = "window"
    type = "S"
  }

  # TTL for automatic cleanup of old rate limit entries
  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  # Point-in-time recovery for production
  point_in_time_recovery {
    enabled = var.enable_point_in_time_recovery
  }

  # Server-side encryption at rest
  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}
