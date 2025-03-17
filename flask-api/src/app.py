#!/usr/bin/env python3
import os
import json
import logging
import datetime
from flask import Flask, request, jsonify
from auth_helper import require_api_key
from prometheus_client import make_wsgi_app, Counter, Histogram
from werkzeug.middleware.dispatcher import DispatcherMiddleware
import time

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
KAFKA_BROKER = os.environ.get('KAFKA_BROKER', 'localhost:9092')
KAFKA_TOPIC = os.environ.get('KAFKA_TOPIC', 'sensor-data')

# API keys - hardcoded for demo
VALID_API_KEYS = ["test-key-1", "test-key-2"]

# Create Flask app
app = Flask(__name__)

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
    if api_key and api_key in VALID_API_KEYS:
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
@require_api_key
def receive_data():
    start_time = time.time()
    data = request.get_json()
    
    # Record the request
    REQUEST_COUNT.labels('secure_api', '/api/data', 'POST', 200).inc()
    
    # Record request latency
    REQUEST_LATENCY.labels('secure_api', '/api/data').observe(time.time() - start_time)
    
    return jsonify({"status": "success", "received": data}), 200

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    print("Starting secure API on port 5000")
    print(f"Authentication: ENABLED")
    print(f"Valid API keys: {len(VALID_API_KEYS)}")
    app.run(host='0.0.0.0', port=port)
