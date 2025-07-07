#!/bin/bash

# Script to sync charts with caching and incremental updates
set -e

echo "Syncing charts with caching..."

# Define paths
SOURCE_CHARTS_DIR="charts/firedoor"
TARGET_CHARTS_DIR="../charts/firedoor"
CACHE_FILE="../charts/.last-sync-hash"
CHANGED_FILES_LOG="../charts/.changed-files.log"

# Function to calculate hash of charts directory
calculate_charts_hash() {
    local dir="$1"
    find "$dir" -type f -exec sha256sum {} \; | sort | sha256sum | cut -d' ' -f1
}

# Function to check if file has changed
file_has_changed() {
    local source_file="$1"
    local target_file="$2"
    
    if [ ! -f "$target_file" ]; then
        return 0  # File doesn't exist in target, consider it changed
    fi
    
    if ! cmp -s "$source_file" "$target_file"; then
        return 0  # Files are different
    fi
    
    return 1  # Files are identical
}

# Function to sync single file
sync_file() {
    local source_file="$1"
    local target_file="$2"
    local relative_path="${source_file#$SOURCE_CHARTS_DIR/}"
    
    # Create target directory if it doesn't exist
    mkdir -p "$(dirname "$target_file")"
    
    # Copy file
    cp "$source_file" "$target_file"
    echo "✅ Synced: $relative_path"
    echo "$relative_path" >> "$CHANGED_FILES_LOG"
}

# Function to remove files that no longer exist in source
cleanup_removed_files() {
    if [ ! -d "$TARGET_CHARTS_DIR" ]; then
        return
    fi
    
    echo "Checking for removed files..."
    find "$TARGET_CHARTS_DIR" -type f | while read -r target_file; do
        relative_path="${target_file#$TARGET_CHARTS_DIR/}"
        source_file="$SOURCE_CHARTS_DIR/$relative_path"
        
        if [ ! -f "$source_file" ]; then
            rm -f "$target_file"
            echo "🗑️  Removed: $relative_path"
            echo "REMOVED: $relative_path" >> "$CHANGED_FILES_LOG"
        fi
    done
}

# Main sync logic
main() {
    # Check if source directory exists
    if [ ! -d "$SOURCE_CHARTS_DIR" ]; then
        echo "❌ Source charts directory not found: $SOURCE_CHARTS_DIR"
        exit 1
    fi
    
    # Calculate current hash
    CURRENT_HASH=$(calculate_charts_hash "$SOURCE_CHARTS_DIR")
    echo "Current charts hash: $CURRENT_HASH"
    
    # Check if we have a cached hash
    if [ -f "$CACHE_FILE" ]; then
        CACHED_HASH=$(cat "$CACHE_FILE")
        echo "Cached hash: $CACHED_HASH"
        
        if [ "$CACHED_HASH" = "$CURRENT_HASH" ]; then
            echo "✅ No changes detected in charts, sync skipped"
            exit 0
        fi
    else
        echo "No cached hash found, performing full sync"
    fi
    
    # Initialize changed files log
    echo "# Changed files log - $(date)" > "$CHANGED_FILES_LOG"
    
    # Create target directory if it doesn't exist
    mkdir -p "$TARGET_CHARTS_DIR"
    
    # Sync files incrementally
    echo "Performing incremental sync..."
    CHANGED_COUNT=0
    TOTAL_COUNT=0
    
    find "$SOURCE_CHARTS_DIR" -type f | while read -r source_file; do
        relative_path="${source_file#$SOURCE_CHARTS_DIR/}"
        target_file="$TARGET_CHARTS_DIR/$relative_path"
        
        TOTAL_COUNT=$((TOTAL_COUNT + 1))
        
        if file_has_changed "$source_file" "$target_file"; then
            sync_file "$source_file" "$target_file"
            CHANGED_COUNT=$((CHANGED_COUNT + 1))
        else
            echo "⏭️  Skipped (unchanged): $relative_path"
        fi
    done
    
    # Cleanup removed files
    cleanup_removed_files
    
    # Update cache
    echo "$CURRENT_HASH" > "$CACHE_FILE"
    
    echo "✅ Sync completed"
    echo "Total files processed: $TOTAL_COUNT"
    echo "Files changed: $CHANGED_COUNT"
    echo "Cache updated with hash: $CURRENT_HASH"
    
    # Show summary of changes
    if [ -f "$CHANGED_FILES_LOG" ]; then
        echo ""
        echo "Changed files summary:"
        grep -v "^#" "$CHANGED_FILES_LOG" | head -10
        if [ $(grep -v "^#" "$CHANGED_FILES_LOG" | wc -l) -gt 10 ]; then
            echo "... and $(($(grep -v "^#" "$CHANGED_FILES_LOG" | wc -l) - 10)) more files"
        fi
    fi
}

# Run main function
main "$@" 