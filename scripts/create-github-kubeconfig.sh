#!/bin/bash

# Script to generate kubeconfig for GitHub Actions CI/CD
# This creates a limited-privilege kubeconfig safe for public repositories

set -e

NAMESPACE="analytics-platform"
SERVICE_ACCOUNT="github-actions-deployer"
CLUSTER_NAME=$(kubectl config view --minify --output 'jsonpath={..cluster.name}')
SERVER=$(kubectl config view --minify --output 'jsonpath={..cluster.server}')

echo "🔧 Creating CI/CD service account and RBAC..."

# Apply the service account and RBAC
kubectl apply -f k8s/ci-cd-service-account.yaml

echo "⏳ Waiting for secret to be created..."
sleep 5

# For Kubernetes 1.24+, we need to create a token request
echo "🔐 Creating service account token (Kubernetes 1.24+ compatible)..."
TOKEN=$(kubectl create token $SERVICE_ACCOUNT -n $NAMESPACE --duration=8760h)

# Get the CA certificate from the cluster
echo "Getting CA certificate from cluster..."
# Try to get embedded CA cert first
CA_CERT=$(kubectl config view --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

# If no embedded cert, read from file (common with minikube)
if [ -z "$CA_CERT" ]; then
    CA_FILE=$(kubectl config view --raw -o jsonpath='{.clusters[0].cluster.certificate-authority}')
    if [ -f "$CA_FILE" ]; then
        echo "Reading CA certificate from file: $CA_FILE"
        CA_CERT=$(cat "$CA_FILE" | base64 | tr -d '\n')
    else
        echo "::error::Cannot find CA certificate. Trying insecure connection..."
        CA_CERT=""
    fi
fi

echo "🔐 Generating kubeconfig for GitHub Actions..."

# Create kubeconfig
if [ -n "$CA_CERT" ]; then
cat > /tmp/github-actions-kubeconfig << EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: $CA_CERT
    server: $SERVER
  name: $CLUSTER_NAME
contexts:
- context:
    cluster: $CLUSTER_NAME
    namespace: $NAMESPACE
    user: $SERVICE_ACCOUNT
  name: github-actions-context
current-context: github-actions-context
users:
- name: $SERVICE_ACCOUNT
  user:
    token: $TOKEN
EOF
else
# Insecure version for local development
cat > /tmp/github-actions-kubeconfig << EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: $SERVER
  name: $CLUSTER_NAME
contexts:
- context:
    cluster: $CLUSTER_NAME
    namespace: $NAMESPACE
    user: $SERVICE_ACCOUNT
  name: github-actions-context
current-context: github-actions-context
users:
- name: $SERVICE_ACCOUNT
  user:
    token: $TOKEN
EOF
fi

echo "✅ Kubeconfig created at /tmp/github-actions-kubeconfig"
echo ""
echo "🔑 Base64-encoded kubeconfig for GitHub Secret:"
echo "=================================="
cat /tmp/github-actions-kubeconfig | base64
echo "=================================="
echo ""
echo "📋 Copy the base64 output above and paste it as the KUBE_CONFIG secret in GitHub"
echo "🔗 Go to: https://github.com/YuLiu003/scalable-real-time-analytics-platform/settings/secrets/actions"

# Cleanup
rm /tmp/github-actions-kubeconfig
