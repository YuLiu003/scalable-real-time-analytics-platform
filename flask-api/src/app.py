#!/usr/bin/env python3
import os
import json
import logging
import datetime
from flask import Flask, request, jsonify
from auth_helper import require_api_key
from prometheus_client import make_wsgi_app, Counter, Histogram
from werkzeug.middleware.dispatcher import DispatcherMiddleware
from werkzeug.middleware.proxy_fix import ProxyFix
import time
import threading
from collections import defaultdict

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
KAFKA_BROKER = os.environ.get('KAFKA_BROKER', 'localhost:9092')
KAFKA_TOPIC = os.environ.get('KAFKA_TOPIC', 'sensor-data')

# API keys - hardcoded for demo
VALID_API_KEYS = ["test-key-1", "test-key-2"]

# Get tenant mapping from environment variable
tenant_api_key_map = {}
tenant_map_str = os.environ.get('TENANT_API_KEY_MAP', '{}')
try:
    tenant_api_key_map = json.loads(tenant_map_str)
    logger.info(f"Loaded tenant map: {tenant_api_key_map}")
except json.JSONDecodeError:
    logger.warning("Could not parse TENANT_API_KEY_MAP environment variable")
    # Default mapping for backward compatibility
    tenant_api_key_map = {
        "test-key-1": "tenant1",
        "test-key-2": "tenant2"
    }
    logger.info(f"Using default tenant map: {tenant_api_key_map}")

# Enable tenant isolation
ENABLE_TENANT_ISOLATION = os.environ.get('ENABLE_TENANT_ISOLATION', 'true').lower() == 'true'
logger.info(f"Tenant isolation enabled: {ENABLE_TENANT_ISOLATION}")

# Implement rate limiting based on tenant
RATE_LIMITS = {
    'tenant1': 100,  # requests per minute
    'tenant2': 200   # requests per minute
}

# Create Flask app
app = Flask(__name__)
app.wsgi_app = ProxyFix(app.wsgi_app)

# Add prometheus wsgi middleware to route /metrics requests
app.wsgi_app = DispatcherMiddleware(app.wsgi_app, {
    '/metrics': make_wsgi_app()
})

# Define Prometheus metrics
REQUEST_COUNT = Counter(
    'api_request_count', 'App Request Count',
    ['app_name', 'endpoint', 'method', 'http_status']
)
REQUEST_LATENCY = Histogram(
    'api_request_latency_seconds', 'Request latency',
    ['app_name', 'endpoint']
)

# Add tenant-specific metrics
TENANT_REQUEST_COUNT = Counter(
    'tenant_request_count', 'Requests by tenant',
    ['tenant_id', 'endpoint']
)

# Add an in-memory storage for testing
in_memory_data = defaultdict(list)

# For debugging - add some test data
if os.environ.get('POPULATE_TEST_DATA', 'false').lower() == 'true':
    in_memory_data["tenant1"].append({
        "device_id": "test-device-1",
        "temperature": 22.5,
        "humidity": 45,
        "tenant_id": "tenant1"
    })
    in_memory_data["tenant2"].append({
        "device_id": "test-device-2",
        "temperature": 23.5,
        "humidity": 50,
        "tenant_id": "tenant2"
    })
    logger.info("Added test data to in-memory storage")

# Kafka producer function
def produce_message(data):
    try:
        from kafka import KafkaProducer
        producer = KafkaProducer(
            bootstrap_servers=KAFKA_BROKER,
            value_serializer=lambda v: json.dumps(v).encode('utf-8')
        )
        producer.send(KAFKA_TOPIC, data)
        producer.flush()
        producer.close()
        logger.info(f"Message sent to Kafka: {data}")
        return "sent"
    except Exception as e:
        logger.error(f"Failed to send message to Kafka: {str(e)}")
        return f"producer_error: {str(e)}"

@app.route('/health', methods=['GET'])
def health_check():
    return jsonify({"status": "healthy"}), 200

@app.route('/auth-test', methods=['GET'])
def auth_test():
    api_key = request.headers.get('X-API-Key')
    if (api_key and api_key in VALID_API_KEYS):
        return jsonify({
            "status": "success", 
            "message": "Authentication successful",
            "key_received": api_key[:3] + "***"
        })
    else:
        return jsonify({
            "status": "error",
            "message": "Authentication failed: Invalid or missing API key"
        }), 401

@app.route('/api/data', methods=['POST'])
def receive_data():
    """Receive and store data with tenant isolation."""
    start_time = time.time()
    
    # Verify API key
    api_key = request.headers.get('X-API-Key')
    if not api_key or api_key not in VALID_API_KEYS:
        return jsonify({"error": "Invalid API key"}), 401
        
    data = request.get_json()
    
    # Store device_id for proper querying
    if 'device_id' not in data:
        return jsonify({"error": "Missing device_id in data"}), 400
    
    # Add tenant context if isolation is enabled
    if ENABLE_TENANT_ISOLATION:
        tenant_id = tenant_api_key_map.get(api_key)
        if not tenant_id:
            logger.warning(f"Unknown tenant for API key: {api_key}")
            return jsonify({"error": "Unknown tenant"}), 403
        
        # Add tenant ID to data
        data['tenant_id'] = tenant_id
        
        # Store in our in-memory storage for testing
        in_memory_data[tenant_id].append(data)
        logger.info(f"Stored data for tenant {tenant_id}, device {data.get('device_id')}")
        logger.debug(f"Data stored: {data}")
        logger.debug(f"Current tenant data: {dict(in_memory_data)}")
    
    # Send to Kafka for real pipeline
    try:
        produce_message(data)
    except Exception as e:
        logger.error(f"Error sending to Kafka: {str(e)}")
    
    # Record metrics
    REQUEST_COUNT.labels('secure_api', '/api/data', 'POST', 200).inc()
    REQUEST_LATENCY.labels('secure_api', '/api/data').observe(time.time() - start_time)
    
    return jsonify({"status": "success", "received": data}), 200

@app.route('/api/data', methods=['GET'])
def get_data():
    """Get data with tenant isolation."""
    # Verify API key
    api_key = request.headers.get('X-API-Key')
    if not api_key or api_key not in VALID_API_KEYS:
        return jsonify({"error": "Invalid API key"}), 401
    
    # Get device_id from query parameters
    device_id = request.args.get('device_id')
    if not device_id:
        return jsonify({"error": "Missing device_id parameter"}), 400
    
    # Apply tenant isolation
    results = []
    if ENABLE_TENANT_ISOLATION:
        tenant_id = tenant_api_key_map.get(api_key)
        if not tenant_id:
            logger.warning(f"Unknown tenant for API key: {api_key}")
            return jsonify({"error": "Unknown tenant"}), 403
        
        # Log in-memory data for debugging
        logger.info(f"Getting data for tenant {tenant_id}, device {device_id}")
        logger.info(f"Available tenant data: {list(in_memory_data.keys())}")
        if tenant_id in in_memory_data:
            logger.info(f"Tenant {tenant_id} has {len(in_memory_data[tenant_id])} records")
        
        # Check if the device belongs to this tenant
        device_belongs_to_tenant = False
        for item in in_memory_data.get(tenant_id, []):
            if item.get('device_id') == device_id:
                device_belongs_to_tenant = True
                results.append(item)
        
        # If we found records, return them
        if results:
            logger.info(f"Found {len(results)} records for tenant {tenant_id}, device {device_id}")
            return jsonify(results)
        
        # Otherwise check if this device belongs to another tenant
        for other_tenant, tenant_data in in_memory_data.items():
            if other_tenant != tenant_id:
                for item in tenant_data:
                    if item.get('device_id') == device_id:
                        # Found the device but it belongs to another tenant
                        logger.warning(f"Tenant {tenant_id} attempted to access device {device_id} belonging to tenant {other_tenant}")
                        return jsonify({"error": "Not authorized to access this device"}), 403
                        
        # Device not found in any tenant's data
        logger.info(f"No records found for device {device_id}")
        return jsonify([])
    else:
        # Non-tenant mode - access all data
        for tenant_data in in_memory_data.values():
            for item in tenant_data:
                if item.get('device_id') == device_id:
                    results.append(item)
        
        return jsonify(results)

@app.route('/debug/tenant-data', methods=['GET'])
def debug_tenant_data():
    """Debug endpoint to view all tenant data in memory."""
    # Convert defaultdict to dict for JSON serialization
    data_by_tenant = {tenant: data for tenant, data in in_memory_data.items()}
    
    response = {
        "tenant_isolation_enabled": ENABLE_TENANT_ISOLATION,
        "tenant_api_key_map": tenant_api_key_map,
        "tenant_data": data_by_tenant,
        "num_tenants": len(data_by_tenant),
        "total_records": sum(len(data) for data in data_by_tenant.values())
    }
    
    return jsonify(response)

@app.route('/debug/clear-data', methods=['POST'])
def clear_data():
    """Clear all in-memory data (for testing)."""
    global in_memory_data
    in_memory_data.clear()
    return jsonify({"status": "success", "message": "All data cleared"})

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    logger.info(f"Starting secure API on port {port}")
    logger.info(f"Authentication: ENABLED")
    logger.info(f"Valid API keys: {len(VALID_API_KEYS)}")
    logger.info(f"Tenant isolation: {ENABLE_TENANT_ISOLATION}")
    
    app.run(host='0.0.0.0', port=port, debug=True)
