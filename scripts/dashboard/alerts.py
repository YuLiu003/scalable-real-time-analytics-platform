from flask import Blueprint, jsonify, request
import logging

alerts_bp = Blueprint('alerts', __name__)

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Sample alert thresholds
ALERT_THRESHOLDS = {
    'cpu_usage': 80,  # percentage
    'memory_usage': 80,  # percentage
}

@alerts_bp.route('/alerts', methods=['GET'])
def get_alerts():
    # This endpoint would return the current alerts
    # In a real application, this would query a database or in-memory store
    return jsonify({"alerts": []})

@alerts_bp.route('/alerts/thresholds', methods=['GET'])
def get_alert_thresholds():
    return jsonify(ALERT_THRESHOLDS)

@alerts_bp.route('/alerts/thresholds', methods=['POST'])
def set_alert_thresholds():
    data = request.json
    if not data:
        return jsonify({"error": "No data provided"}), 400

    for key in ALERT_THRESHOLDS.keys():
        if key in data:
            ALERT_THRESHOLDS[key] = data[key]
            logger.info(f"Updated threshold for {key} to {data[key]}")

    return jsonify(ALERT_THRESHOLDS), 200