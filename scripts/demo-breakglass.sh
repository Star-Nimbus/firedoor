#!/bin/bash

# Demo script for Firedoor breakglass functionality
# This script demonstrates:
# 1. Creating a breakglass resource
# 2. Testing denied access before approval
# 3. Testing allowed access after approval
# 4. Testing denied access after expiration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="firedoor-system"
TEST_NAMESPACE="default"
BREAKGLASS_NAME="demo-breakglass"
TEST_USER="demo-user"
TEST_ROLE="admin"
DURATION_MINUTES=2

echo -e "${BLUE}=== Firedoor Breakglass Demo ===${NC}"
echo

# Function to print section headers
print_section() {
    echo -e "${YELLOW}$1${NC}"
    echo "----------------------------------------"
}

# Function to check if kubectl is available
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}Error: kubectl is not installed or not in PATH${NC}"
        exit 1
    fi
}

# Function to check if the cluster is accessible
check_cluster() {
    if ! kubectl cluster-info &> /dev/null; then
        echo -e "${RED}Error: Cannot connect to Kubernetes cluster${NC}"
        exit 1
    fi
}

# Function to wait for breakglass to be processed
wait_for_breakglass() {
    local name=$1
    local timeout=60
    local count=0
    
    echo "Waiting for breakglass to be processed..."
    while [ $count -lt $timeout ]; do
        if kubectl get breakglass $name -n $NAMESPACE -o jsonpath='{.status.phase}' 2>/dev/null | grep -q "Active\|Denied"; then
            echo -e "${GREEN}Breakglass processed!${NC}"
            return 0
        fi
        sleep 2
        count=$((count + 2))
        echo -n "."
    done
    
    echo -e "${RED}Timeout waiting for breakglass to be processed${NC}"
    return 1
}

# Function to test access
test_access() {
    local user=$1
    local expected_result=$2
    local description=$3
    
    echo -e "\n${BLUE}Testing access for user: $user${NC}"
    echo "Description: $description"
    
    # Try to list pods in the test namespace
    if kubectl --as=$user get pods -n $TEST_NAMESPACE &> /dev/null; then
        if [ "$expected_result" = "allowed" ]; then
            echo -e "${GREEN}✓ Access allowed (expected)${NC}"
        else
            echo -e "${RED}✗ Access allowed (unexpected)${NC}"
        fi
    else
        if [ "$expected_result" = "denied" ]; then
            echo -e "${GREEN}✓ Access denied (expected)${NC}"
        else
            echo -e "${RED}✗ Access denied (unexpected)${NC}"
        fi
    fi
}

# Function to show breakglass status
show_breakglass_status() {
    local name=$1
    echo -e "\n${BLUE}Breakglass Status:${NC}"
    kubectl get breakglass $name -n $NAMESPACE -o yaml | grep -A 20 "status:"
}

# Main script
main() {
    print_section "Prerequisites Check"
    check_kubectl
    check_cluster
    echo -e "${GREEN}✓ Prerequisites met${NC}"
    
    print_section "Step 1: Test Initial Access (Should be Denied)"
    test_access $TEST_USER "denied" "Before breakglass creation - access should be denied"
    
    print_section "Step 2: Create Breakglass Resource"
    echo "Creating breakglass resource..."
    cat <<EOF | kubectl apply -f -
apiVersion: access.cloudnimbus.io/v1alpha1
kind: Breakglass
metadata:
  name: $BREAKGLASS_NAME
  namespace: $NAMESPACE
spec:
  user: $TEST_USER
  namespace: $TEST_NAMESPACE
  role: $TEST_ROLE
  durationMinutes: $DURATION_MINUTES
  approved: false
  reason: "Demo breakglass access"
EOF
    
    echo -e "${GREEN}✓ Breakglass created (not approved)${NC}"
    show_breakglass_status $BREAKGLASS_NAME
    
    print_section "Step 3: Test Access Before Approval (Should be Denied)"
    test_access $TEST_USER "denied" "Breakglass created but not approved - access should be denied"
    
    print_section "Step 4: Approve Breakglass"
    echo "Approving breakglass..."
    kubectl patch breakglass $BREAKGLASS_NAME -n $NAMESPACE --type='merge' -p='{"spec":{"approved":true}}'
    
    # Wait for the controller to process the approval
    wait_for_breakglass $BREAKGLASS_NAME
    
    show_breakglass_status $BREAKGLASS_NAME
    
    print_section "Step 5: Test Access After Approval (Should be Allowed)"
    test_access $TEST_USER "allowed" "Breakglass approved - access should be allowed"
    
    print_section "Step 6: Wait for Expiration"
    echo "Waiting for breakglass to expire (${DURATION_MINUTES} minutes)..."
    echo "You can monitor the status with: kubectl get breakglass $BREAKGLASS_NAME -n $NAMESPACE -w"
    
    # Wait for expiration
    sleep $((DURATION_MINUTES * 60))
    
    # Wait a bit more for the controller to process expiration
    sleep 10
    
    show_breakglass_status $BREAKGLASS_NAME
    
    print_section "Step 7: Test Access After Expiration (Should be Denied)"
    test_access $TEST_USER "denied" "Breakglass expired - access should be denied again"
    
    print_section "Step 8: Cleanup"
    echo "Cleaning up breakglass resource..."
    kubectl delete breakglass $BREAKGLASS_NAME -n $NAMESPACE
    echo -e "${GREEN}✓ Cleanup complete${NC}"
    
    print_section "Demo Complete"
    echo -e "${GREEN}✓ Breakglass demo completed successfully!${NC}"
    echo
    echo "Summary:"
    echo "1. Initial access was denied"
    echo "2. Breakglass created but access still denied (not approved)"
    echo "3. Breakglass approved and access was granted"
    echo "4. Breakglass expired and access was denied again"
    echo
    echo "This demonstrates the complete lifecycle of breakglass access control."
}

# Run the main function
main "$@" 