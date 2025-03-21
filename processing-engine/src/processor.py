import os
import json
import logging
from kafka import KafkaConsumer, KafkaProducer
from prometheus_client import Counter, Histogram, start_http_server
import time

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
KAFKA_BROKER = os.environ.get('KAFKA_BROKER', 'localhost:9092')
INPUT_TOPIC = os.environ.get('INPUT_TOPIC', 'sensor-data')
OUTPUT_TOPIC = os.environ.get('OUTPUT_TOPIC', 'processed-data')
METRICS_PORT = int(os.environ.get('METRICS_PORT', 8000))

# Define tenant-specific processing parameters
tenant_processing_config = {
    'tenant1': {
        'anomaly_threshold': 3.0,
        'sampling_rate': 1.0
    },
    'tenant2': {
        'anomaly_threshold': 2.5,
        'sampling_rate': 0.5
    }
}

DEFAULT_CONFIG = {
    'anomaly_threshold': 2.0,
    'sampling_rate': 1.0
}

def get_tenant_config(tenant_id):
    """Get tenant-specific configuration or default"""
    return tenant_processing_config.get(tenant_id, DEFAULT_CONFIG)

# Prometheus metrics
REQUEST_COUNT = Counter(
    'process_request_count', 'Processing Request Count',
    ['status']
)

PROCESSING_LATENCY = Histogram(
    'processing_latency_seconds', 'Processing latency',
    ['status']
)

# Add tenant-specific metrics
TENANT_MESSAGES_PROCESSED = Counter(
    'tenant_messages_processed', 
    'Messages processed by tenant',
    ['tenant_id']
)

TENANT_PROCESSING_LATENCY = Histogram(
    'tenant_processing_latency_seconds', 
    'Processing latency by tenant',
    ['tenant_id']
)

def process_message(message):
    """Process a message with tenant context preservation"""
    start_time = time.time()
    
    try:
        # Extract tenant_id from message
        tenant_id = message.get('tenant_id')
        if not tenant_id:
            logger.warning("Message received without tenant_id")
            REQUEST_COUNT.labels('missing_tenant').inc()
            return None
        
        # Get tenant-specific configuration
        config = get_tenant_config(tenant_id)
        
        # Apply sampling rate (skip some messages for tenants with lower sampling)
        if config['sampling_rate'] < 1.0 and random.random() > config['sampling_rate']:
            logger.debug(f"Skipping message due to tenant {tenant_id} sampling rate")
            return None
            
        # Process data based on message type
        device_id = message.get('device_id', 'unknown')
        
        # Example: calculate moving average for temperature
        temperature = message.get('temperature')
        humidity = message.get('humidity')
        
        if temperature is not None and humidity is not None:
            # Simple anomaly detection
            anomaly_score = 0
            
            # Check if temperature is outside normal range
            if temperature > 30 or temperature < 10:
                anomaly_score += 1
                
            # Check if humidity is outside normal range
            if humidity > 80 or humidity < 20:
                anomaly_score += 1
                
            # Add anomaly flag if score exceeds threshold
            is_anomaly = anomaly_score >= config['anomaly_threshold']
            
            # Create processed result (preserving tenant context)
            result = {
                'device_id': device_id,
                'temperature': temperature,
                'humidity': humidity,
                'processed_at': time.time(),
                'is_anomaly': is_anomaly,
                'anomaly_score': anomaly_score,
                'tenant_id': tenant_id  # Preserve tenant context
            }
            
            # Record tenant-specific metrics
            TENANT_MESSAGES_PROCESSED.labels(tenant_id).inc()
            processing_time = time.time() - start_time
            TENANT_PROCESSING_LATENCY.labels(tenant_id).observe(processing_time)
            
            # Record general metrics
            REQUEST_COUNT.labels('success').inc()
            PROCESSING_LATENCY.labels('success').observe(processing_time)
            
            return result
        else:
            logger.warning(f"Missing data fields in message for device {device_id}")
            REQUEST_COUNT.labels('invalid_data').inc()
            return None
            
    except Exception as e:
        logger.error(f"Error processing message: {str(e)}")
        REQUEST_COUNT.labels('error').inc()
        PROCESSING_LATENCY.labels('error').observe(time.time() - start_time)
        return None

def main():
    """Main processing loop"""
    # Start metrics server
    start_http_server(METRICS_PORT)
    logger.info(f"Metrics server started on port {METRICS_PORT}")
    
    # Create Kafka consumer
    consumer = KafkaConsumer(
        INPUT_TOPIC,
        bootstrap_servers=KAFKA_BROKER,
        value_deserializer=lambda m: json.loads(m.decode('utf-8')),
        group_id='processing-engine',
        auto_offset_reset='earliest'
    )
    
    # Create Kafka producer for processed data
    producer = KafkaProducer(
        bootstrap_servers=KAFKA_BROKER,
        value_serializer=lambda v: json.dumps(v).encode('utf-8')
    )
    
    logger.info(f"Processing engine started. Consuming from {INPUT_TOPIC}, producing to {OUTPUT_TOPIC}")
    logger.info(f"Tenant-specific processing enabled for {len(tenant_processing_config)} tenants")
    
    # Process messages
    try:
        for msg in consumer:
            data = msg.value
            logger.debug(f"Received message: {data}")
            
            # Process the message
            result = process_message(data)
            
            # If processing succeeded, send to output topic
            if result:
                producer.send(OUTPUT_TOPIC, result)
                logger.debug(f"Processed and sent message: {result}")
    
    except KeyboardInterrupt:
        logger.info("Shutting down processing engine")
    except Exception as e:
        logger.error(f"Unexpected error: {str(e)}")
    finally:
        # Close Kafka connections
        if consumer:
            consumer.close()
        if producer:
            producer.close()

if __name__ == "__main__":
    main()