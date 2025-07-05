#!/bin/bash

# Script to sync CRD from base to Helm chart template
# This ensures the kubebuilder scaffold marker is preserved

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CHART_CRD_FILE="$PROJECT_ROOT/../charts/firedoor-crds/templates/crds.yaml"
BASE_CRD_FILE="$PROJECT_ROOT/config/crd/bases/access.cloudnimbus.io_breakglasses.yaml"

echo "🔄 Syncing CRD from base to Helm chart..."

# Check if files exist
if [ ! -f "$BASE_CRD_FILE" ]; then
    echo "❌ Base CRD file not found: $BASE_CRD_FILE"
    exit 1
fi

if [ ! -f "$CHART_CRD_FILE" ]; then
    echo "❌ Chart CRD file not found: $CHART_CRD_FILE"
    exit 1
fi

# Create temporary file for the updated chart CRD
TEMP_FILE=$(mktemp)

# Add the Helm conditional and kubebuilder marker
echo "{{- if .Values.crds.install }}" > "$TEMP_FILE"
echo "#+kubebuilder:scaffold:crdkustomizeresource" >> "$TEMP_FILE"

# Add the CRD content with proper indentation
cat "$BASE_CRD_FILE" | sed 's/^/  /' >> "$TEMP_FILE"

# Add the closing Helm conditional
echo "{{- end }}" >> "$TEMP_FILE"

# Replace the chart file
cp "$TEMP_FILE" "$CHART_CRD_FILE"

# Clean up
rm "$TEMP_FILE"

echo "✅ CRD synced successfully to: $CHART_CRD_FILE"
echo "📋 Changes:"
echo "   - Updated CRD schema from base file"
echo "   - Preserved kubebuilder scaffold marker"
echo "   - Maintained Helm template structure" 