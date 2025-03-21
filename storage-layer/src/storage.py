import os
import json
import logging
from datetime import datetime, timedelta
from kafka import KafkaConsumer
import threading
import time
from prometheus_client import start_http_server, Counter, Gauge, Histogram
from flask import Flask, jsonify, request, abort
import sqlite3

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
KAFKA_BROKER = os.environ.get('KAFKA_BROKER', 'localhost:9092')
KAFKA_TOPIC = os.environ.get('KAFKA_TOPIC', 'processed-data')
METRICS_PORT = int(os.environ.get('METRICS_PORT', 8001))
API_PORT = int(os.environ.get('API_PORT', 5002))
DB_PATH = os.environ.get('DB_PATH', '/data/analytics.db')

# Tenant-specific data retention policies
tenant_retention_policies = {
    'tenant1': {'days': 90},
    'tenant2': {'days': 30}
}

DEFAULT_RETENTION_DAYS = 60

# Prometheus metrics
STORAGE_COUNT = Counter(
    'storage_count', 'Storage Operation Count',
    ['operation', 'status']
)

STORAGE_LATENCY = Histogram(
    'storage_latency_seconds', 'Storage Operation Latency',
    ['operation']
)

RECORDS_BY_TENANT = Gauge(
    'records_by_tenant', 'Number of Records by Tenant',
    ['tenant_id']
)

# Flask app for queries
app = Flask(__name__)

def init_database():
    """Initialize the SQLite database with tenant isolation"""
    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        # Create a table that includes tenant_id
        cursor.execute('''
        CREATE TABLE IF NOT EXISTS sensor_data (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            device_id TEXT NOT NULL,
            temperature REAL,
            humidity REAL,
            timestamp REAL NOT NULL,
            is_anomaly INTEGER,
            anomaly_score REAL,
            tenant_id TEXT NOT NULL,
            created_at REAL NOT NULL
        )
        ''')
        
        # Create index on tenant_id for efficient queries
        cursor.execute('CREATE INDEX IF NOT EXISTS idx_tenant_id ON sensor_data (tenant_id)')
        cursor.execute('CREATE INDEX IF NOT EXISTS idx_device_id ON sensor_data (device_id)')
        
        conn.commit()
        conn.close()
        logger.info("Database initialized with tenant isolation")
    except Exception as e:
        logger.error(f"Database initialization error: {str(e)}")

def store_data(data):
    """Store data with tenant isolation"""
    start_time = time.time()
    
    try:
        # Extract tenant_id
        tenant_id = data.get('tenant_id')
        if not tenant_id:
            logger.error("Cannot store data without tenant_id")
            STORAGE_COUNT.labels('insert', 'missing_tenant').inc()
            return False
        
        # Connect to database
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        # Insert data with tenant_id
        cursor.execute('''
        INSERT INTO sensor_data 
        (device_id, temperature, humidity, timestamp, is_anomaly, anomaly_score, tenant_id, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ''', (
            data.get('device_id', 'unknown'),
            data.get('temperature'),
            data.get('humidity'),
            data.get('processed_at', time.time()),
            1 if data.get('is_anomaly', False) else 0,
            data.get('anomaly_score', 0),
            tenant_id,
            time.time()
        ))
        
        conn.commit()
        conn.close()
        
        # Update metrics
        STORAGE_COUNT.labels('insert', 'success').inc()
        STORAGE_LATENCY.labels('insert').observe(time.time() - start_time)
        
        # Update record count gauge
        update_record_counts()
        
        logger.debug(f"Stored data for tenant {tenant_id}, device {data.get('device_id')}")
        return True
        
    except Exception as e:
        logger.error(f"Error storing data: {str(e)}")
        STORAGE_COUNT.labels('insert', 'error').inc()
        return False

def update_record_counts():
    """Update metrics for records by tenant"""
    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        cursor.execute('SELECT tenant_id, COUNT(*) FROM sensor_data GROUP BY tenant_id')
        results = cursor.fetchall()
        conn.close()
        
        for tenant_id, count in results:
            RECORDS_BY_TENANT.labels(tenant_id).set(count)
            
    except Exception as e:
        logger.error(f"Error updating record counts: {str(e)}")

def apply_retention_policies():
    """Apply tenant-specific retention policies"""
    start_time = time.time()
    logger.info("Applying tenant retention policies")
    
    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        # Apply retention policy for each tenant
        for tenant_id, policy in tenant_retention_policies.items():
            retention_days = policy.get('days', DEFAULT_RETENTION_DAYS)
            max_age = time.time() - (retention_days * 24 * 60 * 60)
            
            cursor.execute('''
            DELETE FROM sensor_data 
            WHERE tenant_id = ? AND created_at < ?
            ''', (tenant_id, max_age))
            
            deleted_count = cursor.rowcount
            logger.info(f"Applied {retention_days} day retention policy for tenant {tenant_id}: {deleted_count} records deleted")
            STORAGE_COUNT.labels('retention', 'success').inc()
        
        # Apply default retention policy for tenants not explicitly defined
        default_max_age = time.time() - (DEFAULT_RETENTION_DAYS * 24 * 60 * 60)
        
        placeholders = ','.join(['?'] * len(tenant_retention_policies))
        tenant_list = list(tenant_retention_policies.keys())
        
        if tenant_list:
            cursor.execute(f'''
            DELETE FROM sensor_data 
            WHERE tenant_id NOT IN ({placeholders}) AND created_at < ?
            ''', tenant_list + [default_max_age])
        else:
            cursor.execute('''
            DELETE FROM sensor_data 
            WHERE created_at < ?
            ''', (default_max_age,))
            
        deleted_count = cursor.rowcount
        logger.info(f"Applied default {DEFAULT_RETENTION_DAYS} day retention policy: {deleted_count} records deleted")
        
        conn.commit()
        conn.close()
        
        # Update metrics
        STORAGE_LATENCY.labels('retention').observe(time.time() - start_time)
        update_record_counts()
        
    except Exception as e:
        logger.error(f"Error applying retention policies: {str(e)}")
        STORAGE_COUNT.labels('retention', 'error').inc()

def query_data(tenant_id, query_params):
    """Query data with tenant isolation"""
    start_time = time.time()
    
    try:
        # Connect to database
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row  # Return rows as dictionaries
        cursor = conn.cursor()
        
        # Build query with tenant isolation
        query = "SELECT * FROM sensor_data WHERE tenant_id = ?"
        params = [tenant_id]
        
        # Add device_id filter if provided
        if 'device_id' in query_params:
            query += " AND device_id = ?"
            params.append(query_params['device_id'])
        
        # Add time range filter if provided
        if 'start_time' in query_params:
            query += " AND timestamp >= ?"
            params.append(float(query_params['start_time']))
            
        if 'end_time' in query_params:
            query += " AND timestamp <= ?"
            params.append(float(query_params['end_time']))
            
        # Add anomaly filter if provided
        if 'is_anomaly' in query_params:
            is_anomaly = 1 if query_params['is_anomaly'].lower() == 'true' else 0
            query += " AND is_anomaly = ?"
            params.append(is_anomaly)
            
        # Add limit if provided
        if 'limit' in query_params:
            query += " ORDER BY timestamp DESC LIMIT ?"
            params.append(int(query_params['limit']))
        else:
            query += " ORDER BY timestamp DESC LIMIT 1000"  # Default limit
        
        # Execute query
        cursor.execute(query, params)
        results = [dict(row) for row in cursor.fetchall()]
        conn.close()
        
        # Update metrics
        STORAGE_COUNT.labels('query', 'success').inc()
        STORAGE_LATENCY.labels('query').observe(time.time() - start_time)
        
        return results
        
    except Exception as e:
        logger.error(f"Error querying data: {str(e)}")
        STORAGE_COUNT.labels('query', 'error').inc()
        return {"error": str(e)}

@app.route('/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    return jsonify({"status": "healthy"}), 200

@app.route('/api/data/<tenant_id>', methods=['GET'])
def get_tenant_data(tenant_id):
    """API endpoint to get data for a specific tenant"""
    # Verify API key (simplified for example)
    api_key = request.headers.get('X-API-Key')
    if not api_key:
        return jsonify({"error": "Missing API key"}), 401
    
    # Map API key to tenant and verify access
    tenant_api_key_map = {
        "test-key-1": "tenant1",
        "test-key-2": "tenant2"
    }
    
    if tenant_api_key_map.get(api_key) != tenant_id:
        return jsonify({"error": "Not authorized to access this tenant's data"}), 403
    
    # Query data with tenant isolation
    results = query_data(tenant_id, request.args)
    return jsonify(results)

@app.route('/api/data/<tenant_id>/stats', methods=['GET'])
def get_tenant_stats(tenant_id):
    """API endpoint to get statistics for a specific tenant"""
    # Verify API key (simplified for example)
    api_key = request.headers.get('X-API-Key')
    if not api_key:
        return jsonify({"error": "Missing API key"}), 401
    
    # Map API key to tenant and verify access
    tenant_api_key_map = {
        "test-key-1": "tenant1",
        "test-key-2": "tenant2"
    }
    
    if tenant_api_key_map.get(api_key) != tenant_id:
        return jsonify({"error": "Not authorized to access this tenant's data"}), 403
    
    try:
        # Connect to database
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        # Get record count
        cursor.execute('''
        SELECT COUNT(*) FROM sensor_data WHERE tenant_id = ?
        ''', (tenant_id,))
        record_count = cursor.fetchone()[0]
        
        # Get device count
        cursor.execute('''
        SELECT COUNT(DISTINCT device_id) FROM sensor_data WHERE tenant_id = ?
        ''', (tenant_id,))
        device_count = cursor.fetchone()[0]
        
        # Get anomaly count
        cursor.execute('''
        SELECT COUNT(*) FROM sensor_data WHERE tenant_id = ? AND is_anomaly = 1
        ''', (tenant_id,))
        anomaly_count = cursor.fetchone()[0]
        
        # Get latest timestamp
        cursor.execute('''
        SELECT MAX(timestamp) FROM sensor_data WHERE tenant_id = ?
        ''', (tenant_id,))
        latest_data = cursor.fetchone()[0]
        
        conn.close()
        
        stats = {
            "tenant_id": tenant_id,
            "record_count": record_count,
            "device_count": device_count,
            "anomaly_count": anomaly_count,
            "anomaly_percentage": round((anomaly_count / record_count * 100) if record_count > 0 else 0, 2),
            "latest_data": latest_data,
            "retention_days": tenant_retention_policies.get(tenant_id, {}).get('days', DEFAULT_RETENTION_DAYS)
        }
        
        return jsonify(stats)
        
    except Exception as e:
        logger.error(f"Error getting tenant stats: {str(e)}")
        return jsonify({"error": str(e)}), 500

def consumer_thread():
    """Kafka consumer thread to store incoming data"""
    try:
        consumer = KafkaConsumer(
            KAFKA_TOPIC,
            bootstrap_servers=KAFKA_BROKER,
            value_deserializer=lambda m: json.loads(m.decode('utf-8')),
            group_id='storage-layer',
            auto_offset_reset='earliest'
        )
        
        logger.info(f"Storage consumer started. Consuming from {KAFKA_TOPIC}")
        
        for msg in consumer:
            data = msg.value
            logger.debug(f"Received data: {data}")
            store_data(data)
            
    except Exception as e:
        logger.error(f"Consumer error: {str(e)}")

def retention_thread():
    """Thread to periodically apply retention policies"""
    while True:
        try:
            # Apply retention policies daily
            apply_retention_policies()
            time.sleep(24 * 60 * 60)  # Sleep for 24 hours
        except Exception as e:
            logger.error(f"Retention thread error: {str(e)}")
            time.sleep(60 * 60)  # On error, retry after an hour

def main():
    """Main function to start the storage service"""
    # Initialize database
    init_database()
    
    # Start metrics server
    start_http_server(METRICS_PORT)
    logger.info(f"Metrics server started on port {METRICS_PORT}")
    
    # Start consumer thread
    consumer_thread = threading.Thread(target=consumer_thread, daemon=True)
    consumer_thread.start()
    
    # Start retention thread
    retention_thread = threading.Thread(target=retention_thread, daemon=True)
    retention_thread.start()
    
    # Start API server
    logger.info(f"Starting API server on port {API_PORT}")
    app.run(host='0.0.0.0', port=API_PORT)

if __name__ == "__main__":
    main()