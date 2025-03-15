import os
import json
import logging
from flask import Flask, request, jsonify

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Create the Flask app
app = Flask(__name__)

@app.route('/health')
def health():
    logger.info("Health check requested")
    return jsonify({"status": "healthy"}), 200

@app.route('/api/data', methods=['POST'])
def ingest_data():
    logger.info("Data received")
    data = request.get_json()
    if not data:
        logger.warning("No data provided")
        return jsonify({"status": "error", "message": "No data provided"}), 400
    
    # Just log the data for now
    logger.info(f"Data: {data}")
    
    return jsonify({
        "status": "success", 
        "message": "Data logged successfully",
        "data": data
    })

if __name__ == '__main__':
    logger.info("Starting clean ingestion service on port 5000")
    app.run(host='0.0.0.0', port=5000, debug=True)
