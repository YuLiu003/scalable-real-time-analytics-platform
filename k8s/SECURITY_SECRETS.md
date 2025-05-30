# Security Notice: Dynamic Secret Generation

This file explains the secure approach used for managing secrets in this repository.

## 🔐 Security Strategy

### **Why No Static Secrets in Git:**
- **Public Repository Safety**: No secrets are committed to version control
- **Dynamic Generation**: All secrets are generated at deployment time
- **Environment Isolation**: Each deployment gets unique secrets
- **Zero Credential Exposure**: No predictable passwords or keys

### **How Secrets Are Generated:**

#### **Staging Environment:**
- **Kafka Cluster ID**: Generated using `uuidgen`
- **Grafana Password**: Random 16-character string
- **API Keys**: 64-character hex strings using `openssl rand`
- **Database Passwords**: Random 20-character strings
- **JWT Secrets**: 128-character hex strings

#### **Production Environment:**
- Uses GitHub Secrets for sensitive values
- Inherits from organization/repository secret management
- Environment-specific secret configuration
- Proper rotation and management procedures

### **Benefits:**
1. ✅ **No Secret Leakage**: Nothing sensitive in public repository
2. ✅ **Unique Per Deployment**: Each staging deployment is isolated
3. ✅ **Audit Trail**: Secret generation is logged in CI/CD
4. ✅ **Easy Rotation**: Simply redeploy to get new secrets

### **For Local Development:**
Create your own local secrets using the patterns shown in the deployment workflow:

```bash
# Example local secret generation
kubectl create secret generic local-secrets \
  --from-literal=API_KEY="$(openssl rand -hex 32)" \
  --from-literal=JWT_SECRET="$(openssl rand -hex 64)" \
  -n analytics-platform
```

### **Production Setup:**
Set these GitHub Secrets in your repository:
- `KUBE_CONFIG` or `KUBE_CONFIG_PROD`
- `API_KEY_1_PROD`, `API_KEY_2_PROD`
- `JWT_SECRET_PROD`
- `DB_PASSWORD_PROD_B64`
- `REDIS_PASSWORD_PROD_B64`

### **Staging Setup (FIXED):**
**Required Secret:**
- `KUBE_CONFIG` - Base64-encoded kubeconfig for your Kubernetes cluster

**Optional Secrets (will use generated values if missing):**
- `API_KEY_1`, `API_KEY_2` - If you want specific API keys instead of random ones
- `JWT_SECRET` - If you want a specific JWT secret instead of a generated one

**How to set KUBE_CONFIG (Secure Methods):**

**RECOMMENDED: Create a dedicated service account**
1. Run the script: `./scripts/create-github-kubeconfig.sh`
2. Copy the base64 output from the script
3. Go to your repository on GitHub
4. Settings → Secrets and variables → Actions  
5. Click "New repository secret"
6. Name: `KUBE_CONFIG`
7. Value: Paste the base64-encoded kubeconfig from step 2

**Alternative: Use local development cluster**
```bash
# For local clusters (minikube, kind, Docker Desktop)
kubectl config view --raw | base64
```

**Security Benefits:**
- ✅ Limited namespace access (analytics-platform only)
- ✅ Minimal RBAC permissions (deployment actions only)
- ✅ Revocable and auditable
- ✅ Safe for public repositories

## 🚨 Security Best Practices

1. **Never commit real secrets to git**
2. **Use GitHub Secrets for sensitive values**
3. **Rotate secrets regularly**
4. **Use different secrets per environment**
5. **Monitor secret access and usage**
6. **Use least-privilege access principles**

This approach ensures that our public repository remains secure while still providing a complete deployment experience.
