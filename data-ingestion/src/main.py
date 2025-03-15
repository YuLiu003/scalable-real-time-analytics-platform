#!/usr/bin/env python3

import os
import json
import logging
import datetime
from flask import Flask, request, jsonify
from kafka import KafkaProducer

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
KAFKA_BROKER = os.environ.get('KAFKA_BROKER', 'localhost:9092')
KAFKA_TOPIC = os.environ.get('KAFKA_TOPIC', 'sensor-data')
BYPASS_AUTH = os.environ.get('BYPASS_AUTH', 'false').lower() == 'true'

# Valid API keys
VALID_API_KEYS = ["test-key-1", "test-key-2"]

app = Flask(__name__)

def produce_message(data):
    try:
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

def require_api_key(f):
    def decorated(*args, **kwargs):
        if BYPASS_AUTH:
            return f(*args, **kwargs)
        
        api_key = request.headers.get('X-API-Key')
        if api_key and api_key in VALID_API_KEYS:
            return f(*args, **kwargs)
        else:
            return jsonify({"status": "error", "message": "Invalid or missing API key"}), 401
    decorated.__name__ = f.__name__
    return decorated

@app.route('/api/data', methods=['POST'])
@require_api_key
def ingest_data():
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

@app.route('/health')
def health():
    return jsonify({"status": "healthy"}), 200

if __name__ == '__main__':
    logger.info(f"Starting data ingestion service on port 5000. Kafka broker: {KAFKA_BROKER}, Topic: {KAFKA_TOPIC}")
    app.run(host='0.0.0.0', port=5000)
