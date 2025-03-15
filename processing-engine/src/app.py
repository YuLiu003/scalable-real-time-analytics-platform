from flask import Flask, jsonify
from kafka import KafkaConsumer
from threading import Thread
import json
import os
import time
import numpy as np
import requests
from datetime import datetime

app = Flask(__name__)

# Configuration
KAFKA_BROKER = os.environ.get('KAFKA_BROKER', 'kafka:9092')
KAFKA_TOPIC = os.environ.get('KAFKA_TOPIC', 'sensor-data')
KAFKA_GROUP = os.environ.get('KAFKA_GROUP', 'processing-engine')
KAFKA_ENABLED = os.environ.get('KAFKA_ENABLED', 'true').lower() == 'true'
STORAGE_SERVICE = os.environ.get('STORAGE_SERVICE_URL', 'http://storage-layer-service')

# In-memory storage for processed data (for demonstration)
processed_data = {
    "temperature": {
        "count": 0,
        "sum": 0,
        "min": float('inf'),
        "max": float('-inf'),
        "avg": 0
    },
    "humidity": {
        "count": 0,
        "sum": 0,
        "min": float('inf'),
        "max": float('-inf'),
        "avg": 0
    },
    "devices": {}
}

# Processing status
processing_status = {
    "running": False,
    "messages_processed": 0,
    "last_processed": None,
    "errors": 0
}

def process_message(message):
    """Process a single message and update statistics"""
    try:
        # Parse the message
        data = json.loads(message.value.decode('utf-8'))
        
        # Extract values
        device_id = data.get('device_id')
        temperature = data.get('temperature')
        humidity = data.get('humidity')
        timestamp = data.get('timestamp')
        
        if not all([device_id, temperature, humidity, timestamp]):
            print(f"Invalid message: {data}")
            return False
        
        # Update global stats for temperature
        processed_data["temperature"]["count"] += 1
        processed_data["temperature"]["sum"] += temperature
        processed_data["temperature"]["min"] = min(processed_data["temperature"]["min"], temperature)
        processed_data["temperature"]["max"] = max(processed_data["temperature"]["max"], temperature)
        processed_data["temperature"]["avg"] = processed_data["temperature"]["sum"] / processed_data["temperature"]["count"]
        
        # Update global stats for humidity
        processed_data["humidity"]["count"] += 1
        processed_data["humidity"]["sum"] += humidity
        processed_data["humidity"]["min"] = min(processed_data["humidity"]["min"], humidity)
        processed_data["humidity"]["max"] = max(processed_data["humidity"]["max"], humidity)
        processed_data["humidity"]["avg"] = processed_data["humidity"]["sum"] / processed_data["humidity"]["count"]
        
        # Update device-specific stats
        if device_id not in processed_data["devices"]:
            processed_data["devices"][device_id] = {
                "temperature": {
                    "readings": [],
                    "avg": 0,
                    "min": float('inf'),
                    "max": float('-inf')
                },
                "humidity": {
                    "readings": [],
                    "avg": 0,
                    "min": float('inf'),
                    "max": float('-inf')
                },
                "last_seen": None
            }
        
        # Update device temperature stats
        device = processed_data["devices"][device_id]
        device["temperature"]["readings"].append(temperature)
        device["temperature"]["min"] = min(device["temperature"]["min"], temperature)
        device["temperature"]["max"] = max(device["temperature"]["max"], temperature)
        device["temperature"]["avg"] = np.mean(device["temperature"]["readings"])
        
        # Update device humidity stats
        device["humidity"]["readings"].append(humidity)
        device["humidity"]["min"] = min(device["humidity"]["min"], humidity)
        device["humidity"]["max"] = max(device["humidity"]["max"], humidity)
        device["humidity"]["avg"] = np.mean(device["humidity"]["readings"])
        
        # Keep only recent readings (last 100)
        max_readings = 100
        if len(device["temperature"]["readings"]) > max_readings:
            device["temperature"]["readings"] = device["temperature"]["readings"][-max_readings:]
        if len(device["humidity"]["readings"]) > max_readings:
            device["humidity"]["readings"] = device["humidity"]["readings"][-max_readings:]
            
        # Update last seen
        device["last_seen"] = timestamp
        
        # Store processed data in storage service
        store_data({
            "device_id": device_id,
            "temperature": temperature,
            "humidity": humidity,
            "timestamp": timestamp,
            "processed_at": datetime.now().isoformat(),
            "temperature_stats": {
                "avg": device["temperature"]["avg"],
                "min": device["temperature"]["min"],
                "max": device["temperature"]["max"]
            },
            "humidity_stats": {
                "avg": device["humidity"]["avg"],
                "min": device["humidity"]["min"],
                "max": device["humidity"]["max"]
            }
        })
        
        # Update status
        processing_status["messages_processed"] += 1
        processing_status["last_processed"] = datetime.now().isoformat()
        
        return True
    except Exception as e:
        print(f"Error processing message: {e}")
        processing_status["errors"] += 1
        return False

def store_data(processed_data):
    """Store processed data in the storage service"""
    try:
        if not STORAGE_SERVICE:
            print("Storage service URL not configured, skipping data storage")
            return
            
        response = requests.post(
            f"{STORAGE_SERVICE}/api/store",
            json=processed_data,
            headers={"Content-Type": "application/json"},
            timeout=5
        )
        
        if response.status_code != 200:
            print(f"Failed to store data: {response.text}")
    except Exception as e:
        print(f"Error storing data: {e}")

def kafka_consumer_thread():
    """Background thread to consume messages from Kafka"""
    if not KAFKA_ENABLED:
        print("Kafka is disabled. Consumer thread not started.")
        return
        
    processing_status["running"] = True
    print(f"Starting Kafka consumer for topic {KAFKA_TOPIC}")
    
    try:
        # Create consumer
        consumer = KafkaConsumer(
            KAFKA_TOPIC,
            bootstrap_servers=KAFKA_BROKER,
            group_id=KAFKA_GROUP,
            auto_offset_reset='latest',
            enable_auto_commit=True,
            value_deserializer=lambda m: m
        )
        
        # Process messages
        for message in consumer:
            if not processing_status["running"]:
                break
            process_message(message)
            
    except Exception as e:
        print(f"Error in Kafka consumer: {e}")
        processing_status["running"] = False
        processing_status["errors"] += 1

@app.route('/health', methods=['GET'])
def health_check():
    kafka_status = "disabled"
    if KAFKA_ENABLED:
        kafka_status = "connected" if processing_status["running"] else "error"
        
    return jsonify({
        "status": "ok",
        "service": "processing-engine",
        "kafka": kafka_status,
        "timestamp": datetime.now().isoformat()
    })

@app.route('/', methods=['GET'])
def index():
    return jsonify({
        "status": "ok", 
        "service": "processing-engine", 
        "message": "Data Processing Engine",
        "endpoints": {
            "/": "API information (this endpoint)",
            "/health": "Health check",
            "/api/stats": "Get processed statistics"
        }
    })

@app.route('/api/stats', methods=['GET'])
def get_stats():
    """Return current processing statistics"""
    return jsonify({
        "status": "ok",
        "data": processed_data,
        "processing": processing_status
    })

@app.route('/api/stats/device/<device_id>', methods=['GET'])
def get_device_stats(device_id):
    """Return statistics for a specific device"""
    if device_id not in processed_data["devices"]:
        return jsonify({
            "status": "error",
            "message": f"No data for device {device_id}"
        }), 404
        
    return jsonify({
        "status": "ok",
        "device_id": device_id,
        "data": processed_data["devices"][device_id]
    })

if __name__ == '__main__':
    # Start Kafka consumer in background thread
    if KAFKA_ENABLED:
        consumer_thread = Thread(target=kafka_consumer_thread)
        consumer_thread.daemon = True
        consumer_thread.start()
    
    # Start Flask server
    port = int(os.environ.get('PORT', 5001))
    print(f"Starting Processing Engine service on port {port}...")
    app.run(host='0.0.0.0', port=port, debug=False)
