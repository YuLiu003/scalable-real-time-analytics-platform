#!/bin/bash
# create-network-policy.sh

echo "🔒 Creating or updating network policies..."

# Create network policy if it doesn't exist
if [ ! -f "./k8s/network-policy.yaml" ]; then
  echo "Creating network policy"
  
  cat > ./k8s/network-policy.yaml << 'INNER_EOT'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: analytics-platform
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-secure-api
  namespace: analytics-platform
spec:
  podSelector:
    matchLabels:
      app: secure-api
  policyTypes:
  - Ingress
  ingress:
  - from:
    - ipBlock:
        cidr: 0.0.0.0/0
    ports:
    - protocol: TCP
      port: 5000
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-internal-traffic
  namespace: analytics-platform
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: analytics-platform
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: analytics-platform
    ports:
    - protocol: TCP
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-visualization-access
  namespace: analytics-platform
spec:
  podSelector:
    matchLabels:
      app: visualization
  policyTypes:
  - Ingress
  ingress:
  - from:
    - ipBlock:
        cidr: 0.0.0.0/0
    ports:
    - protocol: TCP
      port: 5003
INNER_EOT

  echo "✅ Created network policy file"
else
  echo "✓ Network policy file already exists"
  
  # Check if the policy is correctly configured
  if ! grep -q "podSelector\|namespaceSelector" ./k8s/network-policy.yaml; then
    echo "⚠️ Existing network policy may not be properly configured. Creating backup and updating..."
    
    # Create backup
    cp ./k8s/network-policy.yaml ./k8s/network-policy.yaml.bak
    
    # Update with proper configuration
    cat > ./k8s/network-policy.yaml << 'INNER_EOT'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: analytics-platform
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-secure-api
  namespace: analytics-platform
spec:
  podSelector:
    matchLabels:
      app: secure-api
  policyTypes:
  - Ingress
  ingress:
  - from:
    - ipBlock:
        cidr: 0.0.0.0/0
    ports:
    - protocol: TCP
      port: 5000
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-internal-traffic
  namespace: analytics-platform
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: analytics-platform
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: analytics-platform
    ports:
    - protocol: TCP
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-visualization-access
  namespace: analytics-platform
spec:
  podSelector:
    matchLabels:
      app: visualization
  policyTypes:
  - Ingress
  ingress:
  - from:
    - ipBlock:
        cidr: 0.0.0.0/0
    ports:
    - protocol: TCP
      port: 5003
INNER_EOT
    
    echo "✅ Updated network policy file (backup saved as network-policy.yaml.bak)"
  else
    echo "✓ Network policy is properly configured"
  fi
fi

echo "✅ Network policy configuration completed!"
