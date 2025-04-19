"""
API Authentication Module

This module provides API key authentication for the analytics platform.
"""

import os
from flask import request, jsonify

def require_api_key(f):
    """
    Decorator to require API key authentication
    """
    def decorated_function(*args, **kwargs):
        api_key = request.headers.get('X-API-Key')
        valid_keys = [
            os.environ.get('API_KEY_1', 'test-key-1'),
            os.environ.get('API_KEY_2', 'test-key-2')
        ]
        
        if not api_key or api_key not in valid_keys:
            return jsonify({"error": "Unauthorized - Invalid API Key"}), 401
            
        return f(*args, **kwargs)
    return decorated_function
