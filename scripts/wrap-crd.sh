#!/bin/bash

# Script to wrap generated CRDs with Helm templating
# This ensures CRDs can be upgraded by Helm while maintaining Kubebuilder as the single source of truth

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
GEN_CRD_DIR="$PROJECT_ROOT/.generated"
TEMPLATE_CRD_DIR="$PROJECT_ROOT/charts/firedoor/templates/crd"

echo "🔄 Wrapping CRDs with Helm templating..."

# Create template directory if it doesn't exist
mkdir -p "$TEMPLATE_CRD_DIR"

# Check if generated CRD directory exists
if [ ! -d "$GEN_CRD_DIR" ]; then
    echo "❌ Generated CRD directory not found: $GEN_CRD_DIR"
    echo "   Run 'make manifests' first to generate CRDs"
    exit 1
fi

# Process each generated CRD file
for f in "$GEN_CRD_DIR"/*.yaml; do
    if [ ! -f "$f" ]; then
        echo "⚠️  No CRD files found in $GEN_CRD_DIR"
        break
    fi
    
    b=$(basename "$f")
    echo "📝 Wrapping $b..."
    
    # Create wrapped CRD with Helm conditional
    {
        echo '{{- if .Values.crds.install }}'
        echo '#+kubebuilder:scaffold:crdkustomizeresource'
        cat "$f"
        echo '{{- end }}'
    } > "$TEMPLATE_CRD_DIR/$b"
    
    echo "✅ Wrapped $b -> $TEMPLATE_CRD_DIR/$b"
done

echo "🎉 CRD wrapping completed successfully!"
echo "📋 Summary:"
echo "   - Generated CRDs wrapped with Helm templating"
echo "   - CRDs will be installed/upgraded by Helm"
echo "   - Maintains Kubebuilder as single source of truth"
echo "   - CRDs can be toggled with .Values.crds.install" 