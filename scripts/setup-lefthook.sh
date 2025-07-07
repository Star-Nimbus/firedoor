#!/bin/bash

# Setup script for Lefthook Git hooks
# This script installs and configures Lefthook for the firedoor project

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Setting up Lefthook for Git hooks...${NC}"

# Check if running on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    echo -e "${RED}This script is designed for macOS${NC}"
    exit 1
fi

# Check if Homebrew is installed
if ! command -v brew &> /dev/null; then
    echo -e "${YELLOW}Homebrew not found. Installing Homebrew...${NC}"
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# Install Lefthook
if ! command -v lefthook &> /dev/null; then
    echo -e "${BLUE}Installing Lefthook...${NC}"
    brew install lefthook
    echo -e "${GREEN}Lefthook installed successfully${NC}"
else
    echo -e "${GREEN}Lefthook is already installed${NC}"
fi

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo -e "${RED}Not in a git repository${NC}"
    exit 1
fi

# Install Git hooks
echo -e "${BLUE}Installing Git hooks...${NC}"
lefthook install

echo -e "${GREEN}Git hooks installed successfully!${NC}"
echo -e "${BLUE}Available hooks:${NC}"
lefthook list

echo ""
echo -e "${YELLOW}You can now commit normally. Hooks will run automatically!${NC}"
echo -e "${YELLOW}To skip hooks temporarily: git commit --no-verify${NC}" 