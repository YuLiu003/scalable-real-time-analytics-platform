#!/usr/bin/env python3
import os
import json
import logging
import datetime
from flask import Flask, request, jsonify

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
    return jsonify({"status": "healthy"})

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
def ingest_data():
    # First check for API key
    api_key = request.headers.get('X-API-Key')
    if not api_key or api_key not in VALID_API_KEYS:
        logger.warning("Authentication failed: Invalid or missing API key")
        return jsonify({
            "status": "error",
            "message": "Authentication failed: Invalid or missing API key"
        }), 401

    # Process the request
    data = request.get_json()
    if not data:
        return jsonify({"status": "error", "message": "No data provided"}), 400
    
    # Add timestamp if not present
    if 'timestamp' not in data:
        data['timestamp'] = datetime.datetime.now().isoformat()
    
    # Send to Kafka
    kafka_status = produce_message(data)
    
    return jsonify({
        "status": "success",
        "message": "Data received successfully",
        "data": data,
        "kafka_status": kafka_status
    })

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    print("Starting secure API on port 5000")
    print(f"Authentication: ENABLED")
    print(f"Valid API keys: {len(VALID_API_KEYS)}")
    app.run(host='0.0.0.0', port=port)
