#!/bin/bash

# version.sh - Script to extract version information for builds
# This script can be used in CI/CD pipelines and local development

set -euo pipefail

# Function to get version from git
get_version() {
    local version=""
    local suffix=""

    if [ -n "${CI:-}" ]; then
        suffix="-ci"
    else
        suffix="-dev"
    fi

    # Check if we're in a git repository
    if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        echo "dev${suffix}"
        return
    fi

    # Try to get version from git tags
    if git describe --tags --exact-match >/dev/null 2>&1; then
        # We're on a tagged commit
        version=$(git describe --tags --exact-match)
        suffix=""
    elif git describe --tags >/dev/null 2>&1; then
        # We're ahead of the latest tag
        version="$(git describe --tags --always --dirty)${suffix}"
    else
        # No tags found, use commit hash
        version=$(git rev-parse --short HEAD)
        if [ -n "$(git status --porcelain)" ]; then
            version="${version}-dirty"
        fi
        version="${version}${suffix}"
    fi

    echo "${version}"
}

# Function to get commit hash
get_commit() {
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        git rev-parse HEAD
    else
        echo "unknown"
    fi
}

# Function to get build date
get_date() {
    date -u +'%Y-%m-%dT%H:%M:%SZ'
}

# Function to get build by information
get_build_by() {
    local build_by=""
    
    # Check if we're in CI
    if [ -n "${CI:-}" ]; then
        if [ -n "${GITHUB_ACTIONS:-}" ]; then
            build_by="github-actions"
        elif [ -n "${GITLAB_CI:-}" ]; then
            build_by="gitlab-ci"
        elif [ -n "${JENKINS_URL:-}" ]; then
            build_by="jenkins"
        else
            build_by="ci"
        fi
    else
        # Local build
        build_by="$(whoami)@$(hostname)"
    fi
    
    echo "${build_by}"
}

# Function to export version information as environment variables
export_version_vars() {
    export VERSION="$(get_version)"
    export COMMIT="$(get_commit)"
    export DATE="$(get_date)"
    export BUILD_BY="$(get_build_by)"
}

# Function to print version information
print_version_info() {
    echo "VERSION=$(get_version)"
    echo "COMMIT=$(get_commit)"
    echo "DATE=$(get_date)"
    echo "BUILD_BY=$(get_build_by)"
}

# Function to generate ldflags for Go builds
generate_ldflags() {
    local pkg_prefix="${1:-github.com/cloud-nimbus/firedoor/cmd/cli}"
    
    local version="$(get_version)"
    local commit="$(get_commit)"
    local date="$(get_date)"
    local build_by="$(get_build_by)"
    
    echo "-s -w -X ${pkg_prefix}.Version=${version} -X ${pkg_prefix}.Commit=${commit} -X ${pkg_prefix}.Date=${date} -X ${pkg_prefix}.BuildBy=${build_by}"
}

# Main function
main() {
    case "${1:-}" in
        "version")
            get_version
            ;;
        "commit")
            get_commit
            ;;
        "date")
            get_date
            ;;
        "build-by")
            get_build_by
            ;;
        "export")
            export_version_vars
            ;;
        "print")
            print_version_info
            ;;
        "ldflags")
            generate_ldflags "${2:-}"
            ;;
        *)
            echo "Usage: $0 {version|commit|date|build-by|export|print|ldflags [package]}"
            echo ""
            echo "Commands:"
            echo "  version   - Get version string"
            echo "  commit    - Get commit hash"
            echo "  date      - Get build date"
            echo "  build-by  - Get build by information"
            echo "  export    - Export version variables"
            echo "  print     - Print all version information"
            echo "  ldflags   - Generate ldflags for Go builds"
            exit 1
            ;;
    esac
}

# Run main function if script is executed directly
if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    main "$@"
fi 