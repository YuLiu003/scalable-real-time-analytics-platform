# HashiCorp Vault Integration for Secret Management

This document outlines how to integrate the Real-Time Analytics Platform with HashiCorp Vault for secure secret management in production environments.

## Overview

HashiCorp Vault provides a secure, centralized solution for managing secrets, tokens, and other sensitive data. 
By integrating Vault with our Kubernetes-based analytics platform, we can:

- Centrally manage secrets across environments
- Implement dynamic credentials with automatic rotation
- Provide an audit trail for secret access
- Apply fine-grained access controls
- Meet compliance requirements

## Architecture

![Vault Integration Architecture](../assets/vault-architecture.png)

Our integration uses the Kubernetes Auth Method and the Vault Kubernetes Operator to provide:

1. **Authentication**: Pods authenticate to Vault using their Kubernetes service account
2. **Authorization**: RBAC policies in Vault control which secrets each application can access
3. **Injection**: Secrets are injected as environment variables or mounted as files

## Prerequisites

- HashiCorp Vault server (v1.12+)
- Kubernetes cluster with RBAC enabled
- Helm 3+
- `vault` CLI installed

## Installation

### 1. Install the Vault Operator

```bash
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update

helm install vault-operator hashicorp/vault-operator \
  --namespace vault-system \
  --create-namespace
```

### 2. Configure Kubernetes Authentication in Vault

```bash
# Set your Vault address
export VAULT_ADDR=https://vault.example.com

# Login to Vault
vault login

# Enable Kubernetes auth method
vault auth enable kubernetes

# Configure Kubernetes auth
vault write auth/kubernetes/config \
  kubernetes_host="https://kubernetes.default.svc" \
  kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
  token_reviewer_jwt=@/var/run/secrets/kubernetes.io/serviceaccount/token
```

### 3. Create Vault Policies for Applications

```bash
# Create policy for data ingestion service
cat <<EOF > data-ingestion-policy.hcl
path "secret/data/analytics-platform/data-ingestion/*" {
  capabilities = ["read"]
}
EOF

vault policy write data-ingestion-policy data-ingestion-policy.hcl

# Create similar policies for other services
```

### 4. Create Kubernetes Auth Roles

```bash
# Create role for data ingestion service
vault write auth/kubernetes/role/data-ingestion \
  bound_service_account_names=data-ingestion-sa \
  bound_service_account_namespaces=analytics-platform \
  policies=data-ingestion-policy \
  ttl=1h
```

## Storing Secrets in Vault

Store our application secrets in Vault:

```bash
# API keys
vault kv put secret/analytics-platform/api-keys \
  API_KEY_1="your-secure-api-key-1" \
  API_KEY_2="your-secure-api-key-2" \
  ADMIN_API_KEY="your-secure-admin-api-key"

# Database credentials
vault kv put secret/analytics-platform/database \
  DB_USERNAME="analytics_user" \
  DB_PASSWORD="secure-database-password"

# Store other secrets similarly
```

## Consuming Secrets in Kubernetes

### Option 1: Using Vault Agent Injector

Update the deployment to use Vault Agent Injector:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: data-ingestion-go
  namespace: analytics-platform
  annotations:
    vault.hashicorp.com/agent-inject: "true"
    vault.hashicorp.com/agent-inject-secret-api-keys: "secret/data/analytics-platform/api-keys"
    vault.hashicorp.com/agent-inject-template-api-keys: |
      {{- with secret "secret/data/analytics-platform/api-keys" -}}
      export API_KEY_1="{{ .Data.data.API_KEY_1 }}"
      export API_KEY_2="{{ .Data.data.API_KEY_2 }}"
      {{- end -}}
    vault.hashicorp.com/role: "data-ingestion"
spec:
  # ... rest of deployment spec
```

### Option 2: Using Vault Secrets Operator

Create a VaultStaticSecret resource:

```yaml
apiVersion: secrets.hashicorp.com/v1beta1
kind: VaultStaticSecret
metadata:
  name: api-keys
  namespace: analytics-platform
spec:
  vaultAuthRef: vault-auth
  mount: secret
  path: analytics-platform/api-keys
  destination:
    name: api-keys
    create: true
  refreshAfter: 10s
```

## Environment-Specific Secrets

We maintain different secret paths for staging and production:

- Staging: `secret/analytics-platform/staging/*`
- Production: `secret/analytics-platform/production/*`

## Rotating Secrets

1. Update the secret in Vault with a new value
2. The Vault Agent or Operator will automatically update the Kubernetes resources
3. Services will receive the new credentials without downtime

```bash
# Rotate API key
vault kv put secret/analytics-platform/api-keys \
  API_KEY_1="new-secure-api-key-1" \
  API_KEY_2="your-secure-api-key-2" \
  ADMIN_API_KEY="your-secure-admin-api-key"
```

## Security Considerations

1. **Least Privilege**: Each service has access only to the secrets it needs
2. **Audit Trail**: All access to secrets is logged
3. **Encryption**: Secrets are encrypted at rest and in transit
4. **Rotation**: Regular rotation of secrets reduces risk

## Troubleshooting

Check Vault agent logs:

```bash
kubectl logs deployment/data-ingestion-go -c vault-agent -n analytics-platform
```

Verify service account permissions:

```bash
kubectl auth can-i get secret --as=system:serviceaccount:analytics-platform:data-ingestion-sa -n analytics-platform
```

## Conclusion

This integration ensures our platform's secrets are securely managed and automatically rotated, meeting the requirements for a production-ready system. 