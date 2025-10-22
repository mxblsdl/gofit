#!/bin/bash
set -e

# Create deploy directory
mkdir -p deploy/static
mkdir -p deploy/templates

# Build binary
go build -o deploy/fitbit-server

# Copy assets
cp -r static/* deploy/static/
cp -r templates/* deploy/templates/
cp .env deploy/

echo "Deployment package created in ./deploy"