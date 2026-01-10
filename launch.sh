#!/bin/bash

DEV=false
PROD=false

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
  -d, --dev     Start in development mode
  -p, --prod    Build and run production deployment
  -h, --help    Show this help message
EOF
    exit 0
}

while [[ $# -gt 0 ]];do
    case $1 in
        -d|--dev)
        DEV=true
        shift 
        ;;
        -p|--prod)
        PROD=true
        shift
        ;;
        -h|--help)
        usage
        ;;
        *)
        echo "Unknown option: $1"
        exit 1
        ;;
    esac
done

if [ "$DEV" = false ] && [ "$PROD" = false ] ; then
    usage
fi

if [ "$DEV" = true ] ; then
    echo "Starting in development mode..."
    templ generate --watch &
    air -c .air.toml
    exit 0
fi
if [ "$PROD" = true ] ; then
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
    ./deploy/fitbit-server
    exit 0
fi
