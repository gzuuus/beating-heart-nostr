#!/bin/bash

# install.sh - Installation script for Beating Heart Nostr RAG system dependencies
# This script installs Go, Bun.sh, and Ollama

set -e  # Exit immediately if a command exits with a non-zero status

# Function to print colored output
print_status() {
  local color="\033[0;32m"  # Green
  local reset="\033[0m"
  echo -e "${color}[+] $1${reset}"
}

print_error() {
  local color="\033[0;31m"  # Red
  local reset="\033[0m"
  echo -e "${color}[!] $1${reset}" >&2
}

# Check if script is run as root
if [ "$EUID" -ne 0 ]; then
  print_error "Please run this script as root or with sudo"
  exit 1
fi

# Install prerequisites
print_status "Installing prerequisites..."
apt-get update
apt-get install -y curl wget tar git

# Install Go
install_go() {
  print_status "Installing Go..."
  
  # Set Go version
  GO_VERSION="1.24.1"
  
  # Remove any previous Go installation
  rm -rf /usr/local/go
  
  # Download Go
  wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz -O /tmp/go.tar.gz
  
  # Extract Go to /usr/local
  tar -C /usr/local -xzf /tmp/go.tar.gz
  
  # Add Go to PATH for all users
  echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
  
  # Make the script executable
  chmod +x /etc/profile.d/go.sh
  
  # Clean up
  rm /tmp/go.tar.gz
  
  # Source the profile to update PATH for current session
  source /etc/profile.d/go.sh
  
  # Verify Go installation
  if go version &> /dev/null; then
    print_status "Go $(go version) installed successfully"
  else
    print_error "Go installation failed"
    exit 1
  fi
}

# Install Bun.sh
install_bun() {
  print_status "Installing Bun.sh..."
  
  # Create a temporary user for the installation
  # (Bun installer doesn't work well when run as root)
  if ! id -u buninstaller &>/dev/null; then
    useradd -m -s /bin/bash buninstaller
  fi
  
  # Run the Bun installer as the temporary user
  su - buninstaller -c "curl -fsSL https://bun.sh/install | bash"
  
  # Make Bun available system-wide
  if [ -f /home/buninstaller/.bun/bin/bun ]; then
    cp -r /home/buninstaller/.bun /usr/local/
    ln -sf /usr/local/.bun/bin/bun /usr/local/bin/bun
    
    # Add Bun's global bin directory to PATH for all users
    echo 'export PATH="$PATH:/usr/local/.bun/bin"' > /etc/profile.d/bun.sh
    
    # Make the script executable
    chmod +x /etc/profile.d/bun.sh
    
    # Source the profile to update PATH for current session
    source /etc/profile.d/bun.sh
    
    # Verify Bun installation
    if bun --version &> /dev/null; then
      print_status "Bun $(bun --version) installed successfully"
      
      # Install @dvmcp/bridge globally
      print_status "Installing @dvmcp/bridge package globally..."
      bun install -g @dvmcp/bridge
      
      if [ $? -eq 0 ]; then
        print_status "@dvmcp/bridge installed successfully"
      else
        print_error "@dvmcp/bridge installation failed"
      fi
    else
      print_error "Bun installation failed"
    fi
  else
    print_error "Bun installation failed"
  fi
}

# Install Ollama
install_ollama() {
  print_status "Installing Ollama..."
  
  # Install Ollama
  curl -fsSL https://ollama.com/install.sh | sh
  
  # Verify Ollama installation
  if ollama --version &> /dev/null; then
    print_status "Ollama installed successfully"
    
    # Pull the required model for the RAG system
    print_status "Pulling nomic-embed-text model (this may take a while)..."
    ollama pull nomic-embed-text
  else
    print_error "Ollama installation failed"
    exit 1
  fi
}

# Main installation process
main() {
  print_status "Starting installation of Beating Heart Nostr RAG system dependencies..."
  
  install_go
  install_bun
  install_ollama
  
  print_status "All dependencies installed successfully!"
  print_status "Please log out and log back in, or run 'source /etc/profile.d/go.sh' to use Go in the current session."
  print_status "You can now run the RAG system with: 'go run .' to create the database and 'go run . -mcp' to start the MCP server."
}

# Run the main function
main
