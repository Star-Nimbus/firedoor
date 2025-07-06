# Breakglass Flow Makefile
# This Makefile provides targets to execute a complete breakglass access flow

# Configuration
BREAKGLASS_NAME ?= emergency-access-request
NAMESPACE ?= default
FIREDOOR_NAMESPACE ?= firedoor-system

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: help create-request check-status revoke-access cleanup run-flow show-logs describe

# Default target
help: ## Show this help message
	@echo "$(BLUE)Breakglass Flow Makefile$(NC)"
	@echo ""
	@echo "$(YELLOW)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(YELLOW)Configuration:$(NC)"
	@echo "  BREAKGLASS_NAME=$(BREAKGLASS_NAME)"
	@echo "  NAMESPACE=$(NAMESPACE)"
	@echo "  FIREDOOR_NAMESPACE=$(FIREDOOR_NAMESPACE)"

create-request: ## Create a breakglass access request
	@echo "$(BLUE)Creating breakglass request...$(NC)"
	@kubectl apply -f 01-create-request.yaml
	@echo "$(GREEN)✓ Breakglass request created$(NC)"
	@echo "$(YELLOW)Waiting for controller to process...$(NC)"
	@sleep 5
	@make check-status

check-status: ## Check the status of the breakglass request
	@echo "$(BLUE)Checking breakglass status...$(NC)"
	@kubectl get breakglass $(BREAKGLASS_NAME) -n $(NAMESPACE) -o yaml | grep -A 20 "status:" || echo "$(YELLOW)Status not yet available$(NC)"
	@echo ""
	@echo "$(BLUE)Current conditions:$(NC)"
	@kubectl get breakglass $(BREAKGLASS_NAME) -n $(NAMESPACE) -o jsonpath='{.status.conditions[*].type}: {.status.conditions[*].status}' 2>/dev/null || echo "$(YELLOW)Conditions not yet available$(NC)"
	@echo ""
	@echo "$(BLUE)Phase:$(NC)"
	@kubectl get breakglass $(BREAKGLASS_NAME) -n $(NAMESPACE) -o jsonpath='{.status.phase}' 2>/dev/null || echo "$(YELLOW)Phase not yet available$(NC)"
	@echo ""

revoke-access: ## Revoke the breakglass access
	@echo "$(BLUE)Revoking breakglass access...$(NC)"
	@kubectl apply -f 03-revoke-access.yaml
	@echo "$(GREEN)✓ Revocation request applied$(NC)"
	@echo "$(YELLOW)Waiting for controller to process revocation...$(NC)"
	@sleep 5
	@make check-status

cleanup: ## Remove the breakglass resource
	@echo "$(BLUE)Cleaning up breakglass resource...$(NC)"
	@kubectl delete breakglass $(BREAKGLASS_NAME) -n $(NAMESPACE) --ignore-not-found=true
	@echo "$(GREEN)✓ Breakglass resource cleaned up$(NC)"

run-flow: ## Execute the complete breakglass flow
	@echo "$(BLUE)Starting complete breakglass flow...$(NC)"
	@echo "$(YELLOW)Step 1: Creating request$(NC)"
	@make create-request
	@echo ""
	@echo "$(YELLOW)Step 2: Waiting for approval and activation...$(NC)"
	@sleep 10
	@make check-status
	@echo ""
	@echo "$(YELLOW)Step 3: Simulating work period...$(NC)"
	@echo "$(BLUE)Access is now active. Simulating 30 seconds of work...$(NC)"
	@sleep 30
	@echo ""
	@echo "$(YELLOW)Step 4: Revoking access$(NC)"
	@make revoke-access
	@echo ""
	@echo "$(YELLOW)Step 5: Final status check$(NC)"
	@sleep 5
	@make check-status
	@echo ""
	@echo "$(GREEN)✓ Complete breakglass flow executed$(NC)"

show-logs: ## Show Firedoor operator logs
	@echo "$(BLUE)Showing Firedoor operator logs...$(NC)"
	@kubectl logs -n $(FIREDOOR_NAMESPACE) -l app.kubernetes.io/name=firedoor --tail=50 -f

describe: ## Describe the breakglass resource
	@echo "$(BLUE)Describing breakglass resource...$(NC)"
	@kubectl describe breakglass $(BREAKGLASS_NAME) -n $(NAMESPACE)

events: ## Show events related to the breakglass resource
	@echo "$(BLUE)Showing events for breakglass resource...$(NC)"
	@kubectl get events -n $(NAMESPACE) --field-selector involvedObject.name=$(BREAKGLASS_NAME) --sort-by='.lastTimestamp'

wait-for-approval: ## Wait for the breakglass request to be approved
	@echo "$(BLUE)Waiting for breakglass approval...$(NC)"
	@while true; do \
		STATUS=$$(kubectl get breakglass $(BREAKGLASS_NAME) -n $(NAMESPACE) -o jsonpath='{.status.phase}' 2>/dev/null); \
		if [ "$$STATUS" = "Active" ]; then \
			echo "$(GREEN)✓ Breakglass access is now active$(NC)"; \
			break; \
		elif [ "$$STATUS" = "Denied" ]; then \
			echo "$(RED)✗ Breakglass access was denied$(NC)"; \
			break; \
		else \
			echo "$(YELLOW)Waiting for approval... Current status: $$STATUS$(NC)"; \
			sleep 5; \
		fi; \
	done

wait-for-revocation: ## Wait for the breakglass access to be revoked
	@echo "$(BLUE)Waiting for breakglass revocation...$(NC)"
	@while true; do \
		STATUS=$$(kubectl get breakglass $(BREAKGLASS_NAME) -n $(NAMESPACE) -o jsonpath='{.status.phase}' 2>/dev/null); \
		if [ "$$STATUS" = "Revoked" ] || [ "$$STATUS" = "Expired" ]; then \
			echo "$(GREEN)✓ Breakglass access has been revoked$(NC)"; \
			break; \
		else \
			echo "$(YELLOW)Waiting for revocation... Current status: $$STATUS$(NC)"; \
			sleep 5; \
		fi; \
	done

# Utility targets
validate: ## Validate the YAML manifests
	@echo "$(BLUE)Validating YAML manifests...$(NC)"
	@kubectl apply --dry-run=client -f 01-create-request.yaml
	@kubectl apply --dry-run=client -f 03-revoke-access.yaml
	@echo "$(GREEN)✓ All manifests are valid$(NC)"

check-crd: ## Check if the Breakglass CRD is installed
	@echo "$(BLUE)Checking if Breakglass CRD is installed...$(NC)"
	@kubectl get crd breakglasses.access.cloudnimbus.io >/dev/null 2>&1 && echo "$(GREEN)✓ Breakglass CRD is installed$(NC)" || echo "$(RED)✗ Breakglass CRD is not installed$(NC)"

check-operator: ## Check if the Firedoor operator is running
	@echo "$(BLUE)Checking if Firedoor operator is running...$(NC)"
	@kubectl get pods -n $(FIREDOOR_NAMESPACE) -l app.kubernetes.io/name=firedoor --no-headers | grep -q Running && echo "$(GREEN)✓ Firedoor operator is running$(NC)" || echo "$(RED)✗ Firedoor operator is not running$(NC)"

prerequisites: ## Check all prerequisites
	@echo "$(BLUE)Checking prerequisites...$(NC)"
	@make check-crd
	@make check-operator
	@echo "$(BLUE)Prerequisites check complete$(NC)" 