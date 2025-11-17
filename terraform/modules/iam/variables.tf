variable "name_prefix" {
  description = "Prefix for IAM resource names"
  type        = string
}

variable "dynamodb_table_arn" {
  description = "ARN of the DynamoDB table for rate limiting"
  type        = string
}

variable "tags" {
  description = "Tags to apply to IAM resources"
  type        = map(string)
  default     = {}
}
