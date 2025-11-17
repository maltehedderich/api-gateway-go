# Makefile for API Gateway Terraform Deployment

.PHONY: help init plan apply destroy build clean fmt validate

# Default target
.DEFAULT_GOAL := help

# Colors for output
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
NC     := \033[0m # No Color

help: ## Show this help message
	@echo "$(GREEN)API Gateway - Terraform Deployment$(NC)"
	@echo ""
	@echo "$(YELLOW)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

init: ## Initialize Terraform
	@echo "$(GREEN)Initializing Terraform...$(NC)"
	terraform init

plan: build ## Build Lambda functions and plan Terraform changes
	@echo "$(GREEN)Planning Terraform changes...$(NC)"
	terraform plan

apply: build ## Build Lambda functions and apply Terraform configuration
	@echo "$(GREEN)Applying Terraform configuration...$(NC)"
	terraform apply

apply-auto: build ## Build and apply without confirmation (use with caution)
	@echo "$(YELLOW)Auto-applying Terraform configuration...$(NC)"
	terraform apply -auto-approve

destroy: ## Destroy all Terraform resources
	@echo "$(RED)Destroying Terraform resources...$(NC)"
	@echo "$(RED)WARNING: This will delete all data!$(NC)"
	terraform destroy

build: ## Build Lambda functions
	@echo "$(GREEN)Building Lambda functions...$(NC)"
	./scripts/build-lambdas.sh

clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	find lambda-src -type f -name "bootstrap" -delete
	rm -rf .terraform/tmp

fmt: ## Format Terraform files
	@echo "$(GREEN)Formatting Terraform files...$(NC)"
	terraform fmt -recursive

validate: ## Validate Terraform configuration
	@echo "$(GREEN)Validating Terraform configuration...$(NC)"
	terraform validate

output: ## Show Terraform outputs
	@echo "$(GREEN)Terraform outputs:$(NC)"
	@terraform output

output-json: ## Show Terraform outputs in JSON format
	@terraform output -json

state-list: ## List resources in Terraform state
	@terraform state list

state-show: ## Show detailed state of a resource (usage: make state-show RESOURCE=aws_lambda_function.user)
	@terraform state show $(RESOURCE)

logs-api: ## Tail API Gateway logs (requires AWS CLI)
	@echo "$(GREEN)Tailing API Gateway logs...$(NC)"
	@LOG_GROUP=$$(terraform output -raw api_id 2>/dev/null | xargs -I {} echo "/aws/apigateway/api-gateway-dev"); \
	aws logs tail "$$LOG_GROUP" --follow

logs-lambda: ## Tail Lambda logs for a service (usage: make logs-lambda SERVICE=user-service)
	@echo "$(GREEN)Tailing Lambda logs for $(SERVICE)...$(NC)"
	@aws logs tail "/aws/lambda/api-gateway-dev-$(SERVICE)" --follow

test-health: ## Test health endpoint
	@echo "$(GREEN)Testing health endpoint...$(NC)"
	@API_ENDPOINT=$$(terraform output -raw api_endpoint 2>/dev/null); \
	curl -s "$$API_ENDPOINT/_health" | jq .

test-auth: ## Test authenticated endpoint (requires JWT_TOKEN environment variable)
	@echo "$(GREEN)Testing authenticated endpoint...$(NC)"
	@if [ -z "$(JWT_TOKEN)" ]; then \
		echo "$(RED)Error: JWT_TOKEN environment variable not set$(NC)"; \
		echo "Usage: make test-auth JWT_TOKEN=your_jwt_token"; \
		exit 1; \
	fi
	@API_ENDPOINT=$$(terraform output -raw api_endpoint 2>/dev/null); \
	curl -s -H "Authorization: Bearer $(JWT_TOKEN)" "$$API_ENDPOINT/api/v1/users" | jq .

cost-estimate: ## Estimate monthly AWS costs (requires infracost)
	@echo "$(GREEN)Estimating AWS costs...$(NC)"
	@if ! command -v infracost &> /dev/null; then \
		echo "$(RED)Error: infracost not installed$(NC)"; \
		echo "Install from: https://www.infracost.io/docs/"; \
		exit 1; \
	fi
	terraform plan -out=tfplan.binary
	terraform show -json tfplan.binary > tfplan.json
	infracost breakdown --path tfplan.json
	rm -f tfplan.binary tfplan.json

workspace-list: ## List Terraform workspaces
	@terraform workspace list

workspace-new: ## Create new workspace (usage: make workspace-new NAME=staging)
	@terraform workspace new $(NAME)

workspace-select: ## Select workspace (usage: make workspace-select NAME=staging)
	@terraform workspace select $(NAME)

graph: ## Generate Terraform dependency graph
	@echo "$(GREEN)Generating Terraform dependency graph...$(NC)"
	terraform graph | dot -Tpng > terraform-graph.png
	@echo "Graph saved to terraform-graph.png"

refresh: ## Refresh Terraform state
	@echo "$(GREEN)Refreshing Terraform state...$(NC)"
	terraform refresh

import: ## Import existing AWS resource (usage: make import RESOURCE=aws_lambda_function.user ID=function-name)
	@terraform import $(RESOURCE) $(ID)

taint: ## Mark resource for recreation (usage: make taint RESOURCE=aws_lambda_function.user)
	@terraform taint $(RESOURCE)

untaint: ## Remove taint from resource (usage: make untaint RESOURCE=aws_lambda_function.user)
	@terraform untaint $(RESOURCE)

unlock: ## Force unlock Terraform state (usage: make unlock LOCK_ID=abc-123)
	@terraform force-unlock $(LOCK_ID)

upgrade: ## Upgrade Terraform providers
	@echo "$(GREEN)Upgrading Terraform providers...$(NC)"
	terraform init -upgrade

docs: ## Generate Terraform documentation (requires terraform-docs)
	@echo "$(GREEN)Generating Terraform documentation...$(NC)"
	@if ! command -v terraform-docs &> /dev/null; then \
		echo "$(RED)Error: terraform-docs not installed$(NC)"; \
		echo "Install from: https://terraform-docs.io/"; \
		exit 1; \
	fi
	terraform-docs markdown table . > TERRAFORM_DOCS.md
	@echo "Documentation saved to TERRAFORM_DOCS.md"

check: fmt validate ## Format and validate Terraform configuration
	@echo "$(GREEN)Terraform configuration is valid$(NC)"

full-deploy: clean build init plan apply ## Full deployment (clean, build, init, plan, apply)
	@echo "$(GREEN)Deployment complete!$(NC)"
	@$(MAKE) output
