from flask import Flask, request, jsonify
import os
import json
import sys
from datetime import datetime

# Add the current directory to the path so we can import the tenant model
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from src.tenant_model import Tenant

app = Flask(__name__)

# In-memory tenant store for testing
tenants = {}

@app.route('/api/tenants', methods=['GET'])
def list_tenants():
    """List all tenants."""
    return jsonify({"tenants": [
        {
            "tenant_id": t.tenant_id,
            "name": t.name,
            "plan": t.plan,
            "created_at": t.created_at.isoformat()
        } for t in tenants.values()
    ]})

@app.route('/api/tenants/<tenant_id>', methods=['GET'])
def get_tenant(tenant_id):
    """Get tenant by ID."""
    tenant = tenants.get(tenant_id)
    if not tenant:
        return jsonify({"error": "Tenant not found"}), 404
    
    return jsonify({
        "tenant_id": tenant.tenant_id,
        "name": tenant.name,
        "plan": tenant.plan,
        "resource_limits": tenant.resource_limits,
        "created_at": tenant.created_at.isoformat()
    })

@app.route('/api/tenants', methods=['POST'])
def create_tenant():
    """Create a new tenant."""
    data = request.json
    tenant_id = data.get('tenant_id')
    name = data.get('name')
    plan = data.get('plan', 'basic')
    
    # Validate input
    if not tenant_id or not name:
        return jsonify({"error": "Missing required fields"}), 400
        
    # Check if tenant already exists
    if tenant_id in tenants:
        return jsonify({"error": "Tenant ID already exists"}), 409
    
    # Create tenant
    tenant = Tenant(tenant_id, name, plan)
    tenants[tenant_id] = tenant
    
    return jsonify({
        "tenant_id": tenant.tenant_id,
        "name": tenant.name,
        "plan": tenant.plan,
        "created_at": tenant.created_at.isoformat(),
        "status": "created"
    }), 201

@app.route('/api/tenants/<tenant_id>', methods=['DELETE'])
def delete_tenant(tenant_id):
    """Delete a tenant."""
    if tenant_id not in tenants:
        return jsonify({"error": "Tenant not found"}), 404
    
    del tenants[tenant_id]
    return jsonify({"status": "deleted", "tenant_id": tenant_id})

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    return jsonify({"status": "healthy"}), 200

if __name__ == '__main__':
    # Create default tenants
    tenants['tenant1'] = Tenant('tenant1', 'Demo Corp', 'premium')
    tenants['tenant2'] = Tenant('tenant2', 'Test Inc', 'basic')
    
    # Print tenant info for debugging
    print("Created default tenants:")
    for tenant_id, tenant in tenants.items():
        print(f"- {tenant_id}: {tenant.name} ({tenant.plan})")
    
    app.run(host='0.0.0.0', port=5010, debug=True)