#!/usr/bin/env python3
"""
Test script to verify the real-time analytics platform is working correctly.
This checks connectivity to all components and sends test data through the pipeline.
"""

import os
import sys
import time
import json
import requests
import datetime
import random
from kafka import KafkaProducer, KafkaConsumer
import pandas as pd

# Configuration
API_HOST = os.environ.get("API_HOST", "localhost")
API_PORT = os.environ.get("API_PORT", "5000")
API_KEY = os.environ.get("API_KEY", "test-key-1")
KAFKA_HOST = os.environ.get("KAFKA_HOST", "localhost")
KAFKA_PORT = os.environ.get("KAFKA_PORT", "9092")
TEST_TOPIC = "test-analytics-data"

print("🔍 Testing Real-Time Analytics Platform")
print("======================================")

# Check API health
print("\n1️⃣ Checking API health...")
try:
    health_url = f"http://{API_HOST}:{API_PORT}/health"
    response = requests.get(health_url, timeout=5)
    if response.status_code == 200:
        print("✅ API health check successful")
        print(f"   Response: {response.json()}")
    else:
        print(f"❌ API health check failed with status code: {response.status_code}")
        print(f"   Response: {response.text}")
except Exception as e:
    print(f"❌ API health check failed with error: {e}")
    print("   This might happen if the API is not accessible. Check if the service is running.")

# Test API authentication
print("\n2️⃣ Testing API authentication...")
try:
    auth_test_url = f"http://{API_HOST}:{API_PORT}/api/data"
    # Test without auth
    print("   Testing without API key...")
    no_auth_response = requests.post(auth_test_url, json={"test": "data"}, timeout=5)
    if no_auth_response.status_code == 401:
        print("✅ Authentication working correctly (rejected unauthenticated request)")
    else:
        print(f"❌ Authentication check failed: unauthenticated request returned {no_auth_response.status_code}")
    
    # Test with auth
    print("   Testing with API key...")
    auth_response = requests.post(
        auth_test_url, 
        headers={"X-API-Key": API_KEY, "Content-Type": "application/json"},
        json={"test": "data"},
        timeout=5
    )
    if auth_response.status_code == 200:
        print("✅ Authentication working correctly (accepted authenticated request)")
        print(f"   Response: {auth_response.json()}")
    else:
        print(f"❌ Authentication failed with status code: {auth_response.status_code}")
        print(f"   Response: {auth_response.text}")
except Exception as e:
    print(f"❌ API authentication test failed with error: {e}")

# Test Kafka connectivity
print("\n3️⃣ Testing Kafka connectivity...")
try:
    kafka_bootstrap_servers = f"{KAFKA_HOST}:{KAFKA_PORT}"
    print(f"   Connecting to Kafka at {kafka_bootstrap_servers}...")
    
    # Create a producer
    producer = KafkaProducer(
        bootstrap_servers=[kafka_bootstrap_servers],
        value_serializer=lambda v: json.dumps(v).encode('utf-8'),
        retries=3
    )
    print("✅ Successfully connected to Kafka (producer)")
    
    # Send test message
    test_message = {
        "device_id": "test-device",
        "timestamp": datetime.datetime.now().isoformat(),
        "temperature": round(random.uniform(20.0, 30.0), 2),
        "humidity": round(random.uniform(30.0, 60.0), 2)
    }
    print(f"   Sending test message to topic '{TEST_TOPIC}'...")
    future = producer.send(TEST_TOPIC, test_message)
    result = future.get(timeout=10)
    producer.flush()
    print(f"✅ Message sent successfully to partition {result.partition} at offset {result.offset}")
    
    # Create a consumer
    print("   Creating consumer to verify message delivery...")
    consumer = KafkaConsumer(
        TEST_TOPIC,
        bootstrap_servers=[kafka_bootstrap_servers],
        auto_offset_reset='latest',
        enable_auto_commit=True,
        group_id='test-group',
        value_deserializer=lambda x: json.loads(x.decode('utf-8'))
    )
    
    # Poll for message
    print("   Waiting for message...")
    received = False
    start_time = time.time()
    timeout = 10  # seconds
    consumer.poll(0)  # Get consumer going
    
    while time.time() - start_time < timeout and not received:
        for msg in consumer:
            print(f"✅ Received message: {msg.value}")
            received = True
            break
        time.sleep(0.1)
        
    if not received:
        print("❌ No message received within timeout period")
    
    consumer.close()
    producer.close()
    
except Exception as e:
    print(f"❌ Kafka connectivity test failed with error: {e}")

# Test TimeSeries Database
print("\n4️⃣ Testing Time Series Database...")
try:
    sys.path.append('/Users/yuliu/real-time-analytics-platform/storage-layer/src')
    from db.timeseries import TimeSeriesDatabase
    
    # Use an in-memory SQLite for testing
    test_db = TimeSeriesDatabase('sqlite:///:memory:')
    
    # Insert test data
    now = datetime.datetime.now()
    print("   Inserting test data...")
    test_db.insert_data(now, 25.5, 'temperature')
    test_db.insert_data(now - datetime.timedelta(minutes=5), 24.8, 'temperature')
    test_db.insert_data(now - datetime.timedelta(minutes=10), 24.2, 'temperature')
    
    # Query data back
    print("   Querying test data...")
    results = test_db.query_data(
        now - datetime.timedelta(minutes=15),
        now + datetime.timedelta(minutes=1),
        'temperature'
    )
    
    if len(results) == 3:
        print(f"✅ Time series database working correctly. Retrieved {len(results)} records.")
        print("   Sample data:")
        print(results.head())
    else:
        print(f"❌ Time series database test failed. Expected 3 records but got {len(results)}.")
        
except Exception as e:
    print(f"❌ Time Series Database test failed with error: {e}")
    print("   This test requires the Python modules to be in the expected location.")

print("\n======================================")
print("🏁 Platform Testing Complete")
print("   Check the results above to verify all components are working correctly.")