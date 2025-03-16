FROM python:3.10-slim

WORKDIR /app

# Create empty requirements file if none exists
RUN mkdir -p /app/src && touch /app/requirements.txt

# Copy requirements file if it exists (won't fail if missing)
COPY flask-api/requirements.txt /app/requirements.txt 2>/dev/null || :

# Install dependencies
RUN pip install --no-cache-dir -r requirements.txt flask requests

# Copy source code
COPY flask-api/src/ /app/src/

# Create health check endpoint file
RUN echo 'from flask import Flask, request, jsonify\nfrom auth_helper import require_api_key\n\napp = Flask(__name__)\n\n@app.route("/health", methods=["GET"])\ndef health_check():\n    return jsonify({"status": "healthy"}), 200\n\n@app.route("/api/data", methods=["POST"])\n@require_api_key\ndef receive_data():\n    data = request.get_json()\n    return jsonify({"status": "success", "received": data}), 200\n\nif __name__ == "__main__":\n    app.run(host="0.0.0.0", port=5000)' > /app/src/app.py

# Make sure we have the auth_helper.py file
RUN echo 'from flask import request\nimport os\n\ndef require_api_key(f):\n    def decorated_function(*args, **kwargs):\n        api_key = request.headers.get("X-API-Key")\n        valid_keys = [\n            os.environ.get("API_KEY_1", "test-key-1"),\n            os.environ.get("API_KEY_2", "test-key-2")\n        ]\n        \n        if not api_key or api_key not in valid_keys:\n            return {"error": "Unauthorized - Invalid API Key"}, 401\n            \n        return f(*args, **kwargs)\n    return decorated_function' > /app/src/auth_helper.py

EXPOSE 5000

CMD ["python", "-m", "src.app"]
