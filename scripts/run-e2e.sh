#!/bin/bash

# E2E Test Runner Script for Firedoor
# This script helps run e2e tests with proper setup and debugging

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check if skaffold is available
    if ! command -v skaffold &> /dev/null; then
        print_error "skaffold is not installed or not in PATH"
        exit 1
    fi
    
    # Check if go is available
    if ! command -v go &> /dev/null; then
        print_error "go is not installed or not in PATH"
        exit 1
    fi
    
    print_success "All prerequisites are available"
}

# Check cluster connection
check_cluster() {
    print_status "Checking cluster connection..."
    
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster"
        print_error "Please ensure you have a cluster running and kubectl is configured"
        exit 1
    fi
    
    print_success "Connected to Kubernetes cluster"
}

# Deploy operator if not already deployed
deploy_operator() {
    print_status "Checking if operator is deployed..."
    
    if ! kubectl get pods -n firedoor-system -l control-plane=controller-manager &> /dev/null; then
        print_status "Operator not found, deploying with skaffold..."
        if skaffold run --profile=dev; then
            print_success "Operator deployed successfully"
        else
            print_error "Failed to deploy operator"
            exit 1
        fi
    else
        print_success "Operator is already deployed"
    fi
}

# Wait for operator to be ready
wait_for_operator() {
    print_status "Waiting for operator to be ready..."
    
    timeout=300  # 5 minutes
    interval=5   # 5 seconds
    
    for ((i=0; i<timeout; i+=interval)); do
        if kubectl get pods -n firedoor-system -l control-plane=controller-manager -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q "Running"; then
            print_success "Operator is ready"
            return 0
        fi
        print_status "Waiting for operator to be ready... ($i/$timeout seconds)"
        sleep $interval
    done
    
    print_error "Operator failed to become ready within $timeout seconds"
    return 1
}

# Wait for CRD to be available
wait_for_crd() {
    print_status "Waiting for breakglass CRD to be available..."
    
    timeout=120  # 2 minutes
    interval=5   # 5 seconds
    
    for ((i=0; i<timeout; i+=interval)); do
        if kubectl get crd breakglasses.access.cloudnimbus.io &> /dev/null; then
            print_success "Breakglass CRD is available"
            return 0
        fi
        print_status "Waiting for CRD... ($i/$timeout seconds)"
        sleep $interval
    done
    
    print_error "CRD failed to become available within $timeout seconds"
    return 1
}

# Run e2e tests
run_tests() {
    print_status "Running e2e tests..."
    
    # Run the tests
    if go test ./test/e2e/ -v -ginkgo.v; then
        print_success "E2E tests passed!"
    else
        print_error "E2E tests failed!"
        
        # Show operator logs for debugging
        print_status "Showing operator logs for debugging..."
        kubectl logs -n firedoor-system -l control-plane=controller-manager --tail=50 || true
        
        # Show recent breakglass resources
        print_status "Showing recent breakglass resources..."
        kubectl get breakglasses -n firedoor-system -o yaml || true
        
        exit 1
    fi
}

# Cleanup function
cleanup() {
    print_status "Cleaning up..."
    
    # Delete test breakglass resources
    kubectl delete breakglasses --all -n firedoor-system --ignore-not-found=true || true
    
    print_success "Cleanup completed"
}

# Main execution
main() {
    print_status "Starting Firedoor E2E Test Runner"
    
    # Set up cleanup on exit
    trap cleanup EXIT
    
    # Run checks and setup
    check_prerequisites
    check_cluster
    deploy_operator
    wait_for_operator
    wait_for_crd
    
    # Run tests
    run_tests
}

# Run main function
main "$@" 