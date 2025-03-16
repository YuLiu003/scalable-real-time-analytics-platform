#!/bin/bash
# configure-api-auth.sh

echo "🔒 Configuring API authentication..."

# Check if secure-api deployment exists
if [ -f "./k8s/secure-api-deployment.yaml" ]; then
  echo "Updating secure-api-deployment.yaml with API key configuration"
  
  # Create a temporary file
  tmp_file=$(mktemp)
  
  # Process the file to add API key environment variables
  awk '
  /env:/ {
    print $0;
    print "        - name: API_KEY_1";
    print "          valueFrom:";
    print "            secretKeyRef:";
    print "              name: api-keys";
    print "              key: api-key-1";
    print "        - name: API_KEY_2";
    print "          valueFrom:";
    print "            secretKeyRef:";
    print "              name: api-keys";
    print "              key: api-key-2";
    next;
  }
  { print $0; }
  ' "./k8s/secure-api-deployment.yaml" > "$tmp_file"
  
  # Replace the original file
  mv "$tmp_file" "./k8s/secure-api-deployment.yaml"
  
  echo "✅ Updated secure-api-deployment.yaml with API key configuration"
else
  echo "⚠️ secure-api-deployment.yaml not found"
  
  # Look for alternative deployment file names
  api_deployment=$(find ./k8s/ -name "*api*deployment*.yaml" | head -1)
  
  if [ -n "$api_deployment" ]; then
    echo "Found alternative API deployment: $api_deployment"
    
    # Create a temporary file
    tmp_file=$(mktemp)
    
    # Process the file to add API key environment variables
    awk '
    /env:/ {
      print $0;
      print "        - name: API_KEY_1";
      print "          valueFrom:";
      print "            secretKeyRef:";
      print "              name: api-keys";
      print "              key: api-key-1";
      print "        - name: API_KEY_2";
      print "          valueFrom:";
      print "            secretKeyRef:";
      print "              name: api-keys";
      print "              key: api-key-2";
      next;
    }
    { print $0; }
    ' "$api_deployment" > "$tmp_file"
    
    # Replace the original file
    mv "$tmp_file" "$api_deployment"
    
    echo "✅ Updated $api_deployment with API key configuration"
  else
    echo "❌ No API deployment file found. Creating a new one..."
    
    # Create a basic secure-api deployment
    mkdir -p ./k8s
    cat > ./k8s/secure-api-deployment.yaml << 'INNER_EOT'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secure-api
  namespace: analytics-platform
spec:
  replicas: 1
  selector:
    matchLabels:
      app: secure-api
  template:
    metadata:
      labels:
        app: secure-api
    spec:
      securityContext:
        runAsUser: 1000
        runAsGroup: 3000
        fsGroup: 2000
      containers:
      - name: secure-api
        image: secure-api:latest
        ports:
        - containerPort: 5000
        env:
        - name: API_KEY_1
          valueFrom:
            secretKeyRef:
              name: api-keys
              key: api-key-1
        - name: API_KEY_2
          valueFrom:
            secretKeyRef:
              name: api-keys
              key: api-key-2
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
        resources:
          limits:
            cpu: "500m"
            memory: "512Mi"
          requests:
            cpu: "100m"
            memory: "128Mi"
        livenessProbe:
          httpGet:
            path: /health
            port: 5000
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health
            port: 5000
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 2
          successThreshold: 1
          failureThreshold: 3
INNER_EOT
    
    echo "✅ Created new secure-api-deployment.yaml with API key configuration"
  fi
fi

# Also ensure a Kubernetes secret is created for API keys
cat > ./k8s/api-keys-secret.yaml << 'INNER_EOT'
apiVersion: v1
kind: Secret
metadata:
  name: api-keys
  namespace: analytics-platform
type: Opaque
data:
  api-key-1: dGVzdC1rZXktMQ==  # Base64 encoded "test-key-1"
  api-key-2: dGVzdC1rZXktMg==  # Base64 encoded "test-key-2"
INNER_EOT

echo "✅ Created api-keys-secret.yaml"
echo "⚠️ Note: For production, replace the API keys with secure values"

# Add API key check to flask-api code if it exists
if [ -d "./flask-api/src" ]; then
  echo "Adding API key authentication to Flask API code"
  
  # Check if app.py exists
  app_file=$(find ./flask-api/src -name "app.py" | head -1)
  
  if [ -n "$app_file" ]; then
    # Create a temporary file
    tmp_file=$(mktemp)
    
    # Add API key authentication if not already present
    if ! grep -q "X-API-Key" "$app_file"; then
      cat > "$tmp_file" << 'INNER_EOT'
import os
import functools
from flask import Flask, request, jsonify

app = Flask(__name__)

# API key authentication
def require_api_key(f):
    @functools.wraps(f)
    def decorated_function(*args, **kwargs):
        api_key = request.headers.get('X-API-Key')
        valid_keys = [
            os.environ.get('API_KEY_1', 'test-key-1'),
            os.environ.get('API_KEY_2', 'test-key-2')
        ]
        
        if not api_key or api_key not in valid_keys:
            return jsonify({"error": "Unauthorized - Invalid API Key"}), 401
            
        return f(*args, **kwargs)
    return decorated_function

# Add a health check endpoint
@app.route('/health')
def health():
    return jsonify({"status": "healthy"}), 200

# Example protected endpoint
@app.route('/api/data', methods=['POST'])
@require_api_key
def receive_data():
    data = request.json
    # Process the data...
    return jsonify({"status": "success", "message": "Data received"}), 200

# Authentication test endpoint
@app.route('/auth-test')
@require_api_key
def auth_test():
    return jsonify({"status": "success", "message": "Authentication successful"}), 200

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=int(os.environ.get('PORT', 5000)))
INNER_EOT

      # Replace the original file
      mv "$tmp_file" "$app_file"
      echo "✅ Updated Flask API code with API key authentication"
    else
      echo "✓ API key authentication already present in Flask code"
    fi
  else
    echo "⚠️ Flask app.py not found"
  fi
else
  echo "⚠️ flask-api directory not found"
fi

echo "✅ API authentication configuration completed!"
