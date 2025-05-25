# 🔐 Real-Time Analytics Platform - Secure Setup Guide

## ⚠️ IMPORTANT: Repository Security Status

This repository has been cleaned up and is **SAFE for public access** after implementing the following security measures:

### ✅ **Security Measures Implemented:**

1. **✅ All hardcoded credentials removed**
2. **✅ Secrets converted to templates**
3. **✅ Environment variable-based configuration**
4. **✅ Comprehensive .gitignore patterns**
5. **✅ Secure credential generation scripts**

---

## 🛡️ **Secure Deployment Instructions**

### **Step 1: Generate Secure Credentials**
```bash
# Run the automated secure credential setup
./scripts/setup-secrets.sh
```

This script will:
- Generate cryptographically secure passwords and API keys
- Create Kubernetes secrets in the cluster
- Display credentials for your records (save them securely!)

### **Step 2: Deploy Platform**
```bash
# Build and deploy the platform
./manage.sh setup-all
```

### **Step 3: Access Services**
```bash
# Access Grafana (use credentials from setup-secrets.sh)
./manage.sh grafana

# Access data ingestion API (use generated API keys)
./manage.sh access-api
```

---

## 🔑 **API Key Management**

### **Retrieving API Keys**
```bash
# Get API Key 1
kubectl get secret tenant-management-secrets -n analytics-platform -o jsonpath='{.data.API_KEY_1}' | base64 -d

# Get API Key 2  
kubectl get secret tenant-management-secrets -n analytics-platform -o jsonpath='{.data.API_KEY_2}' | base64 -d
```

### **Using API Keys**
```bash
# Example API call with your secure key
curl -X POST http://localhost:5000/api/data \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_SECURE_API_KEY" \
  -d '{"device_id": "sensor-001", "temperature": 25.5, "humidity": 60}'
```

---

## 🚀 **Production Deployment**

### **For Production Environments:**

1. **Use External Secret Management**
   - HashiCorp Vault
   - AWS Secrets Manager
   - Azure Key Vault
   - Kubernetes External Secrets Operator

2. **Enable Additional Security**
   ```bash
   # Deploy with production security settings
   ./manage.sh deploy-prod
   
   # Run comprehensive security check
   ./scripts/security-check.sh --production
   ```

3. **Network Security**
   - Network policies are automatically applied
   - RBAC configurations included
   - TLS encryption for inter-service communication

---

## 🔍 **Security Monitoring**

### **Built-in Security Features:**
- API key authentication on all endpoints
- Request rate limiting
- Network policies for pod isolation
- RBAC for Kubernetes access control
- Prometheus metrics for security monitoring

### **Grafana Security Dashboards:**
- Authentication failure monitoring
- API usage patterns
- Unusual traffic detection
- Resource access monitoring

---

## 📋 **Security Checklist**

Before deploying to production:

- [ ] Run `./scripts/setup-secrets.sh` to generate secure credentials
- [ ] Verify no hardcoded credentials in code: `./scripts/security-check.sh`
- [ ] Enable GitHub secret scanning
- [ ] Configure external secret management for production
- [ ] Set up monitoring and alerting
- [ ] Review and update network policies
- [ ] Enable audit logging
- [ ] Configure backup encryption

---

## 🆘 **Emergency Procedures**

### **If Credentials Are Compromised:**
```bash
# Rotate all credentials immediately
./scripts/setup-secrets.sh

# Restart all services to use new credentials
kubectl rollout restart deployment -n analytics-platform

# Review access logs
kubectl logs -l app=data-ingestion-go -n analytics-platform --tail=1000
```

---

## 📞 **Support & Security Contact**

For security issues or questions:
- Create a GitHub issue with the `security` label
- For sensitive security matters, email: [security contact]

## 🔗 **Additional Resources**

- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
- [OWASP Application Security Guidelines](https://owasp.org/)
- [Platform Security Architecture Documentation](./docs/)

---

**🎉 This platform is now secure and ready for production deployment!**
