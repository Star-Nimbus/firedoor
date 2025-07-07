#!/bin/bash
set -e
echo "Wrapping CRDs with Helm templating..."

# Define paths
GEN_CRD_DIR=".generated"
TEMPLATE_CRD_DIR="charts/firedoor/templates/crd"

# Check if generated CRD directory exists
if [ ! -d "$GEN_CRD_DIR" ]; then
    echo "Generated CRD directory not found: $GEN_CRD_DIR"
    echo "   Run 'make manifests' first to generate CRDs"
    exit 1
fi

# Create template directory if it doesn't exist
mkdir -p "$TEMPLATE_CRD_DIR"

# Find all CRD files
CRD_FILES=$(find "$GEN_CRD_DIR" -name "*.yaml" -o -name "*.yml")

if [ -z "$CRD_FILES" ]; then
    echo "No CRD files found in $GEN_CRD_DIR"
    echo "   Run 'make manifests' first to generate CRDs"
    exit 1
fi

# Process each CRD file
for b in $CRD_FILES; do
    echo "Wrapping $b..."
    
    # Extract filename
    filename=$(basename "$b")
    
    # Create wrapped version with Helm templating (no kubebuilder reference, no extra indentation)
    {
        echo "{{- if .Values.crds.install }}"
        cat "$b"
        echo "{{- end }}"
    } > "$TEMPLATE_CRD_DIR/$filename"
    
    echo "Wrapped $b -> $TEMPLATE_CRD_DIR/$filename"
done

echo "CRD wrapping completed successfully!"
echo "Summary:"
echo " - Processed $(echo "$CRD_FILES" | wc -w) CRD files"
echo " - Output directory: $TEMPLATE_CRD_DIR"
