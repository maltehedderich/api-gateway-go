output "table_name" {
  description = "DynamoDB table name"
  value       = aws_dynamodb_table.this.name
}

output "table_id" {
  description = "DynamoDB table ID"
  value       = aws_dynamodb_table.this.id
}

output "table_arn" {
  description = "DynamoDB table ARN"
  value       = aws_dynamodb_table.this.arn
}

output "stream_arn" {
  description = "DynamoDB stream ARN"
  value       = aws_dynamodb_table.this.stream_arn
}

output "stream_label" {
  description = "DynamoDB stream label"
  value       = aws_dynamodb_table.this.stream_label
}

output "hash_key" {
  description = "Hash key name"
  value       = aws_dynamodb_table.this.hash_key
}

output "range_key" {
  description = "Range key name"
  value       = aws_dynamodb_table.this.range_key
}
