#!/bin/bash

# Script to sync CRD from base to Helm chart
set -e

echo "Syncing CRD from base to Helm chart..."

# Define paths
BASE_CRD_FILE="api/v1alpha1/breakglass_types.go"
CHART_CRD_FILE="charts/firedoor/templates/crd/access.cloudnimbus.io_breakglasses.yaml"
GEN_CRD_DIR=".generated"

# Check if base CRD file exists
if [ ! -f "$BASE_CRD_FILE" ]; then
    echo "Base CRD file not found: $BASE_CRD_FILE"
    exit 1
fi

# Check if chart CRD file exists
if [ ! -f "$CHART_CRD_FILE" ]; then
    echo "Chart CRD file not found: $CHART_CRD_FILE"
    exit 1
fi

# Create generated directory if it doesn't exist
mkdir -p "$GEN_CRD_DIR"

# Generate CRD from Go types to .generated directory
echo "Generating CRD from Go types..."
controller-gen crd:maxDescLen=0,generateEmbeddedObjectMeta=true paths="./api/v1alpha1/..." output:crd:dir="$GEN_CRD_DIR"

# Check if generation was successful
if [ $? -eq 0 ]; then
    echo "CRD generated successfully to $GEN_CRD_DIR"
    
    # Use the wrap-crd.sh script to properly wrap the CRDs with Helm templating
    echo "Wrapping CRDs with Helm templating..."
    ./scripts/wrap-crd.sh
    
    echo "CRD synced successfully to: $CHART_CRD_FILE"
else
    echo "Failed to sync CRD"
    exit 1
fi 