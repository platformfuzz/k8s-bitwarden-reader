FROM mcr.microsoft.com/devcontainers/go:1.25

# Create user-writable Go directories and set permissions
RUN mkdir -p /home/vscode/go/pkg/mod /home/vscode/.cache/go-build && \
  chown -R vscode:vscode /home/vscode/go /home/vscode/.cache

# Set Go environment variables to use user-writable directories
ENV GOPATH=/home/vscode/go
ENV GOCACHE=/home/vscode/.cache/go-build
ENV GOMODCACHE=/home/vscode/go/pkg/mod

# Install Go tools (goimports, etc.) as vscode user
USER vscode
RUN go install -v golang.org/x/tools/cmd/goimports@latest
USER root

# Install Docker CLI (for docker-in-docker support)
RUN apt-get update && \
  apt-get install -y docker.io && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
  chmod +x kubectl && \
  mv kubectl /usr/local/bin/

# Install make if not present
RUN apt-get update && \
  apt-get install -y make && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*
