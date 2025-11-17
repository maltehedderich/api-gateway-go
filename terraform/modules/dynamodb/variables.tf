variable "table_name" {
  description = "Name of the DynamoDB table"
  type        = string
}

variable "billing_mode" {
  description = "DynamoDB billing mode (PAY_PER_REQUEST or PROVISIONED)"
  type        = string
  default     = "PAY_PER_REQUEST"
}

variable "read_capacity" {
  description = "Read capacity units (only for PROVISIONED billing mode)"
  type        = number
  default     = 5
}

variable "write_capacity" {
  description = "Write capacity units (only for PROVISIONED billing mode)"
  type        = number
  default     = 5
}

variable "enable_point_in_time_recovery" {
  description = "Enable point-in-time recovery"
  type        = bool
  default     = false
}

variable "tags" {
  description = "Tags to apply to the DynamoDB table"
  type        = map(string)
  default     = {}
}
