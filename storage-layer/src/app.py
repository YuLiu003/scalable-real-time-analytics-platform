from flask import Flask, jsonify, request
import os
import json
import time
import sqlite3
from datetime import datetime
import threading

app = Flask(__name__)

# Configuration
DB_PATH = os.environ.get('DB_PATH', '/data/analytics.db')
DATA_DIR = os.path.dirname(DB_PATH)

# Ensure data directory exists
os.makedirs(DATA_DIR, exist_ok=True)

# Database lock for thread safety
db_lock = threading.Lock()

def init_db():
    """Initialize the database schema"""
    with db_lock:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        # Create tables
        cursor.execute('''
        CREATE TABLE IF NOT EXISTS sensor_data (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            device_id TEXT NOT NULL,
            temperature REAL NOT NULL,
            humidity REAL NOT NULL,
            timestamp TEXT NOT NULL,
            processed_at TEXT NOT NULL,
            created_at TEXT NOT NULL
        )
        ''')
        
        cursor.execute('''
        CREATE TABLE IF NOT EXISTS device_stats (
            device_id TEXT PRIMARY KEY,
            temp_min REAL,
            temp_max REAL,
            temp_avg REAL,
            humidity_min REAL,
            humidity_max REAL,
            humidity_avg REAL,
            last_updated TEXT NOT NULL
        )
        ''')
        
        conn.commit()
        conn.close()

def store_sensor_data(data):
    """Store sensor data in the database"""
    with db_lock:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        # Insert sensor data
        cursor.execute(
            'INSERT INTO sensor_data (device_id, temperature, humidity, timestamp, processed_at, created_at) VALUES (?, ?, ?, ?, ?, ?)',
            (
                data['device_id'],
                data['temperature'],
                data['humidity'],
                data['timestamp'],
                data['processed_at'],
                datetime.now().isoformat()
            )
        )
        
        # Update device stats
        temp_stats = data.get('temperature_stats', {})
        humidity_stats = data.get('humidity_stats', {})
        
        cursor.execute(
            '''
            INSERT OR REPLACE INTO device_stats 
            (device_id, temp_min, temp_max, temp_avg, humidity_min, humidity_max, humidity_avg, last_updated)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            ''',
            (
                data['device_id'],
                temp_stats.get('min', data['temperature']),
                temp_stats.get('max', data['temperature']),
                temp_stats.get('avg', data['temperature']),
                humidity_stats.get('min', data['humidity']),
                humidity_stats.get('max', data['humidity']),
                humidity_stats.get('avg', data['humidity']),
                datetime.now().isoformat()
            )
        )
        
        conn.commit()
        conn.close()
        
        return True

def get_recent_data(limit=100):
    """Get recent sensor data"""
    with db_lock:
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        
        cursor.execute(
            'SELECT * FROM sensor_data ORDER BY timestamp DESC LIMIT ?',
            (limit,)
        )
        
        rows = cursor.fetchall()
        result = [{key: row[key] for key in row.keys()} for row in rows]
        
        conn.close()
        return result

def get_device_stats():
    """Get statistics for all devices"""
    with db_lock:
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        
        cursor.execute('SELECT * FROM device_stats')
        
        rows = cursor.fetchall()
        result = {row['device_id']: {key: row[key] for key in row.keys() if key != 'device_id'} for row in rows}
        
        conn.close()
        return result

def get_device_data(device_id, limit=100):
    """Get data for a specific device"""
    with db_lock:
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        
        cursor.execute(
            'SELECT * FROM sensor_data WHERE device_id = ? ORDER BY timestamp DESC LIMIT ?',
            (device_id, limit)
        )
        
        rows = cursor.fetchall()
        result = [{key: row[key] for key in row.keys()} for row in rows]
        
        conn.close()
        return result

@app.route('/health', methods=['GET'])
def health_check():
    # Try to connect to the database
    try:
        conn = sqlite3.connect(DB_PATH)
        conn.close()
        db_status = "connected"
    except Exception as e:
        db_status = f"error: {str(e)}"
    
    return jsonify({
        "status": "ok",
        "service": "storage-layer",
        "database": db_status,
        "timestamp": datetime.now().isoformat()
    })

@app.route('/', methods=['GET'])
def index():
    return jsonify({
        "status": "ok", 
        "service": "storage-layer", 
        "message": "Data Storage Layer",
        "endpoints": {
            "/": "API information (this endpoint)",
            "/health": "Health check",
            "/api/store": "Store data (POST)",
            "/api/data": "Get recent data (GET)",
            "/api/data/<device_id>": "Get data for specific device (GET)",
            "/api/stats": "Get device statistics (GET)"
        }
    })

@app.route('/api/store', methods=['POST'])
def store_data():
    """Store sensor data"""
    try:
        data = request.json
        if not data:
            return jsonify({"status": "error", "message": "No data provided"}), 400
            
        # Validate required fields
        required_fields = ['device_id', 'temperature', 'humidity', 'timestamp']
        for field in required_fields:
            if field not in data:
                return jsonify({
                    "status": "error", 
                    "message": f"Missing required field: {field}"
                }), 400
                
        # Store the data
        store_sensor_data(data)
        
        return jsonify({
            "status": "success",
            "message": "Data stored successfully"
        })
    except Exception as e:
        return jsonify({
            "status": "error",
            "message": f"Failed to store data: {str(e)}"
        }), 500

@app.route('/api/data', methods=['GET'])
def get_data():
    """Get recent sensor data"""
    try:
        limit = request.args.get('limit', 100, type=int)
        data = get_recent_data(limit)
        
        return jsonify({
            "status": "success",
            "count": len(data),
            "data": data
        })
    except Exception as e:
        return jsonify({
            "status": "error",
            "message": f"Failed to retrieve data: {str(e)}"
        }), 500

@app.route('/api/data/<device_id>', methods=['GET'])
def get_device_data_api(device_id):
    """Get data for a specific device"""
    try:
        limit = request.args.get('limit', 100, type=int)
        data = get_device_data(device_id, limit)
        
        return jsonify({
            "status": "success",
            "device_id": device_id,
            "count": len(data),
            "data": data
        })
    except Exception as e:
        return jsonify({
            "status": "error",
            "message": f"Failed to retrieve data: {str(e)}"
        }), 500

@app.route('/api/stats', methods=['GET'])
def get_stats():
    """Get device statistics"""
    try:
        stats = get_device_stats()
        
        return jsonify({
            "status": "success",
            "device_count": len(stats),
            "stats": stats
        })
    except Exception as e:
        return jsonify({
            "status": "error",
            "message": f"Failed to retrieve stats: {str(e)}"
        }), 500

if __name__ == '__main__':
    # Initialize the database
    init_db()
    
    # Start Flask server
    port = int(os.environ.get('PORT', 5002))
    print(f"Starting Storage Layer service on port {port}...")
    app.run(host='0.0.0.0', port=port, debug=False)
