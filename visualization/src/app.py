from flask import Flask, jsonify, render_template_string, request
import requests
import os
import json
import time
from datetime import datetime
from flask_socketio import SocketIO, emit

app = Flask(__name__)
app.config['SECRET_KEY'] = 'real-time-analytics-secret!'
socketio = SocketIO(app, cors_allowed_origins="*")

# Configuration - use environment variables with defaults for local testing
DATA_SERVICE = os.environ.get('DATA_SERVICE_URL', 'http://data-ingestion-service')

# In-memory data store for demo purposes
recent_data = []
MAX_DATA_POINTS = 100

@app.route('/', methods=['GET'])
def index():
    # HTML template with real-time charts using Chart.js
    html_template = """
    <!DOCTYPE html>
    <html>
    <head>
        <title>Real-Time Analytics Platform</title>
        <script src="https://cdn.jsdelivr.net/npm/chart.js@3.7.1/dist/chart.min.js"></script>
        <script src="https://cdn.socket.io/4.4.1/socket.io.min.js"></script>
        <style>
            body { 
                font-family: Arial, sans-serif; 
                margin: 0;
                padding: 0;
                background-color: #f8f9fa;
            }
            .container { 
                max-width: 1200px; 
                margin: 0 auto; 
                padding: 20px;
            }
            .header {
                background-color: #343a40;
                color: white;
                padding: 15px 20px;
                box-shadow: 0 2px 5px rgba(0,0,0,0.1);
                margin-bottom: 20px;
            }
            .header h1 {
                margin: 0;
                font-size: 24px;
            }
            .dashboard {
                display: grid;
                grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
                gap: 20px;
                margin-bottom: 20px;
            }
            .card {
                background: white; 
                border-radius: 5px; 
                padding: 20px; 
                box-shadow: 0 2px 5px rgba(0,0,0,0.1);
            }
            .card-header {
                font-size: 18px;
                font-weight: bold;
                margin-bottom: 15px;
                color: #343a40;
                border-bottom: 1px solid #eee;
                padding-bottom: 10px;
            }
            .chart-container {
                position: relative;
                height: 250px;
                width: 100%;
            }
            .data-form {
                background: white;
                border-radius: 5px;
                padding: 20px;
                box-shadow: 0 2px 5px rgba(0,0,0,0.1);
                margin-bottom: 20px;
            }
            .form-group { 
                margin-bottom: 15px; 
            }
            label { 
                display: block; 
                margin-bottom: 5px; 
                font-weight: bold;
                color: #495057;
            }
            input { 
                padding: 10px; 
                width: 100%; 
                border: 1px solid #ced4da;
                border-radius: 4px;
                box-sizing: border-box;
            }
            button { 
                padding: 10px 15px; 
                background: #007bff; 
                color: white; 
                border: none; 
                border-radius: 4px;
                cursor: pointer; 
            }
            button:hover { 
                background: #0069d9; 
            }
            pre { 
                background: #f8f9fa; 
                padding: 15px; 
                border-radius: 4px; 
                overflow-x: auto; 
                font-size: 14px;
            }
            .device-stats {
                display: grid;
                grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
                gap: 15px;
            }
            .stat-card {
                background: #f8f9fa;
                border-radius: 4px;
                padding: 15px;
                text-align: center;
            }
            .stat-value {
                font-size: 24px;
                font-weight: bold;
                margin: 10px 0;
            }
            .stat-label {
                font-size: 14px;
                color: #6c757d;
            }
            .error { color: #dc3545; }
            .success { color: #28a745; }
            .footer {
                background-color: #343a40;
                color: white;
                padding: 10px 20px;
                font-size: 12px;
                text-align: center;
                margin-top: 40px;
            }
        </style>
    </head>
    <body>
        <div class="header">
            <h1>Real-Time Analytics Platform</h1>
        </div>
        
        <div class="container">
            <div class="dashboard">
                <div class="card">
                    <div class="card-header">Temperature (°C)</div>
                    <div class="chart-container">
                        <canvas id="temperatureChart"></canvas>
                    </div>
                </div>
                
                <div class="card">
                    <div class="card-header">Humidity (%)</div>
                    <div class="chart-container">
                        <canvas id="humidityChart"></canvas>
                    </div>
                </div>
            </div>
            
            <div class="card">
                <div class="card-header">Device Statistics</div>
                <div class="device-stats">
                    <div class="stat-card">
                        <div class="stat-label">Average Temperature</div>
                        <div id="avgTemp" class="stat-value">--</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">Average Humidity</div>
                        <div id="avgHumidity" class="stat-value">--</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">Total Readings</div>
                        <div id="totalReadings" class="stat-value">0</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">Last Reading</div>
                        <div id="lastReading" class="stat-value">--</div>
                    </div>
                </div>
            </div>
            
            <div class="data-form">
                <div class="card-header">Send Test Data</div>
                <div class="form-group">
                    <label for="deviceId">Device ID:</label>
                    <input type="text" id="deviceId" value="device-001">
                </div>
                <div class="form-group">
                    <label for="temperature">Temperature (°C):</label>
                    <input type="number" id="temperature" value="25.5">
                </div>
                <div class="form-group">
                    <label for="humidity">Humidity (%):</label>
                    <input type="number" id="humidity" value="60">
                </div>
                <button onclick="sendData()">Send Data</button>
                <div id="response" style="margin-top: 20px;"></div>
            </div>
            
            <div class="card">
                <div class="card-header">Recent Data</div>
                <pre id="recentData">No data yet</pre>
            </div>
            
            <div class="card">
                <div class="card-header">System Status</div>
                <button onclick="checkStatus()">Check Services Status</button>
                <div id="statusResults" style="margin-top: 20px;"></div>
            </div>
        </div>
        
        <div class="footer">
            Real-Time Analytics Platform | &copy; 2025
        </div>
        
        <script>
            // Initialize socket.io connection
            const socket = io();
            
            // Configuration
            const settings = {
                maxDataPoints: 20,
                updateInterval: 2000  // Update interval in ms
            };
            
            // Chart configuration
            const chartOptions = {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: {
                        ticks: {
                            maxRotation: 0,
                            autoSkip: true,
                            maxTicksLimit: 10
                        }
                    },
                    y: {
                        beginAtZero: false
                    }
                },
                animation: {
                    duration: 500
                },
                plugins: {
                    legend: {
                        display: false
                    }
                }
            };
            
            // Initialize charts
            let temperatureChart, humidityChart;
            
            // Function to format date for display
            function formatTime(date) {
                return date.toLocaleTimeString('en-US', { hour12: false });
            }
            
            // Initialize charts when DOM is loaded
            document.addEventListener('DOMContentLoaded', function() {
                initCharts();
                loadInitialData();
            });
            
            // Initialize the charts
            function initCharts() {
                // Temperature chart
                const tempCtx = document.getElementById('temperatureChart').getContext('2d');
                temperatureChart = new Chart(tempCtx, {
                    type: 'line',
                    data: {
                        labels: [],
                        datasets: [{
                            label: 'Temperature',
                            data: [],
                            borderColor: '#ff6384',
                            backgroundColor: 'rgba(255, 99, 132, 0.1)',
                            borderWidth: 2,
                            tension: 0.2,
                            fill: true
                        }]
                    },
                    options: chartOptions
                });
                
                // Humidity chart
                const humidityCtx = document.getElementById('humidityChart').getContext('2d');
                humidityChart = new Chart(humidityCtx, {
                    type: 'line',
                    data: {
                        labels: [],
                        datasets: [{
                            label: 'Humidity',
                            data: [],
                            borderColor: '#36a2eb',
                            backgroundColor: 'rgba(54, 162, 235, 0.1)',
                            borderWidth: 2,
                            tension: 0.2,
                            fill: true
                        }]
                    },
                    options: chartOptions
                });
            }
            
            // Load initial data
            function loadInitialData() {
                fetch('/api/data/recent')
                .then(response => response.json())
                .then(data => {
                    if (data.data && data.data.length > 0) {
                        // Update stats and charts with initial data
                        updateStats(data.data);
                        
                        // Add each data point to charts
                        data.data.forEach(item => {
                            updateCharts(item);
                        });
                        
                        // Update recent data display
                        document.getElementById('recentData').textContent = 
                            JSON.stringify(data.data, null, 2);
                    }
                })
                .catch(error => console.error('Error loading initial data:', error));
            }
            
            // Handle real-time data updates
            socket.on('data_update', function(newData) {
                // Add the new data point to charts
                updateCharts(newData);
                
                // Add to recent data at the beginning
                const recentDataElem = document.getElementById('recentData');
                let currentData;
                try {
                    currentData = JSON.parse(recentDataElem.textContent);
                    if (!Array.isArray(currentData)) {
                        currentData = [];
                    }
                } catch (e) {
                    currentData = [];
                }
                
                // Add new data at the beginning
                currentData.unshift(newData);
                
                // Keep only recent items
                if (currentData.length > 10) {
                    currentData = currentData.slice(0, 10);
                }
                
                // Update display
                recentDataElem.textContent = JSON.stringify(currentData, null, 2);
                
                // Update statistics
                updateStats(currentData);
            });
            
            // Function to update charts with new data
            function updateCharts(newData) {
                if (!temperatureChart || !humidityChart || !newData.timestamp) return;
                
                const timestamp = new Date(newData.timestamp);
                const timeString = formatTime(timestamp);
                
                // Add new data point to temperature chart
                temperatureChart.data.labels.push(timeString);
                temperatureChart.data.datasets[0].data.push(newData.temperature);
                
                // Add new data point to humidity chart
                humidityChart.data.labels.push(timeString);
                humidityChart.data.datasets[0].data.push(newData.humidity);
                
                // Limit the number of data points
                if (temperatureChart.data.labels.length > settings.maxDataPoints) {
                    temperatureChart.data.labels.shift();
                    temperatureChart.data.datasets[0].data.shift();
                }
                
                if (humidityChart.data.labels.length > settings.maxDataPoints) {
                    humidityChart.data.labels.shift();
                    humidityChart.data.datasets[0].data.shift();
                }
                
                // Update charts
                temperatureChart.update();
                humidityChart.update();
            }
            
            // Update statistics display
            function updateStats(data) {
                if (!Array.isArray(data) || data.length === 0) return;
                
                // Calculate statistics
                const tempValues = data.map(item => item.temperature).filter(val => !isNaN(val));
                const humidityValues = data.map(item => item.humidity).filter(val => !isNaN(val));
                
                const avgTemp = tempValues.length > 0 
                    ? (tempValues.reduce((sum, val) => sum + val, 0) / tempValues.length).toFixed(1)
                    : '--';
                
                const avgHumidity = humidityValues.length > 0
                    ? (humidityValues.reduce((sum, val) => sum + val, 0) / humidityValues.length).toFixed(1)
                    : '--';
                
                // Update the DOM
                document.getElementById('avgTemp').textContent = avgTemp;
                document.getElementById('avgHumidity').textContent = avgHumidity;
                document.getElementById('totalReadings').textContent = data.length;
                
                // Format last reading time
                const lastTime = data[0]?.timestamp 
                    ? formatTime(new Date(data[0].timestamp))
                    : '--';
                document.getElementById('lastReading').textContent = lastTime;
            }
            
            // Function to send data
            function sendData() {
                const deviceId = document.getElementById('deviceId').value;
                const temperature = parseFloat(document.getElementById('temperature').value);
                const humidity = parseFloat(document.getElementById('humidity').value);
                
                if (isNaN(temperature) || isNaN(humidity)) {
                    document.getElementById('response').innerHTML = 
                        '<div class="error">Temperature and humidity must be numeric values</div>';
                    return;
                }
                
                const data = {
                    device_id: deviceId,
                    temperature: temperature,
                    humidity: humidity,
                    timestamp: new Date().toISOString()
                };
                
                const responseElement = document.getElementById('response');
                responseElement.innerHTML = 'Sending data...';
                
                fetch('/api/data', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(data),
                })
                .then(response => response.json())
                .then(data => {
                    responseElement.innerHTML = `
                        <div class="success">Success!</div>
                        <pre>${JSON.stringify(data, null, 2)}</pre>
                    `;
                })
                .catch((error) => {
                    responseElement.innerHTML = `
                        <div class="error">Error: ${error}</div>
                    `;
                });
            }
            
            // Function to check services status
            function checkStatus() {
                const statusElement = document.getElementById('statusResults');
                statusElement.innerHTML = 'Checking services...';
                
                fetch('/api/status')
                .then(response => response.json())
                .then(data => {
                    statusElement.innerHTML = `
                        <pre>${JSON.stringify(data, null, 2)}</pre>
                    `;
                })
                .catch((error) => {
                    statusElement.innerHTML = `
                        <div class="error">Error: ${error}</div>
                    `;
                });
            }
        </script>
    </body>
    </html>
    """
    return render_template_string(html_template)

@app.route('/health', methods=['GET'])
def health_check():
    return jsonify({
        "status": "ok",
        "service": "visualization"
    })

@app.route('/api/status', methods=['GET'])
def system_status():
    services = {
        "visualization": {"status": "ok"},
        "data-ingestion": {"status": "unknown"},
        "processing-engine": {"status": "unknown"},
        "storage-layer": {"status": "unknown"}
    }
    
    # Try to contact the data ingestion service
    try:
        response = requests.get(f"{DATA_SERVICE}/health", timeout=2)
        if response.status_code == 200:
            services["data-ingestion"] = response.json()
    except Exception as e:
        services["data-ingestion"] = {"status": "error", "message": str(e)}
    
    return jsonify({
        "status": "ok",
        "services": services,
        "timestamp": datetime.now().isoformat()
    })

@app.route('/api/data', methods=['POST'])
def proxy_data_ingestion():
    """Proxy the data ingestion request to the data-ingestion service"""
    try:
        data = request.json
        
        # Add timestamp if not provided
        if 'timestamp' not in data:
            data['timestamp'] = datetime.now().isoformat()
        
        # Store in memory for visualization
        global recent_data
        recent_data.insert(0, data)
        if len(recent_data) > MAX_DATA_POINTS:
            recent_data = recent_data[:MAX_DATA_POINTS]
        
        # Emit new data to all connected clients
        socketio.emit('data_update', data)
        
        # Forward the request to the data ingestion service
        try:
            response = requests.post(
                f"{DATA_SERVICE}/api/data",
                json=data,
                headers={"Content-Type": "application/json"},
                timeout=5
            )
            result = response.json()
        except Exception as e:
            # If service is unavailable, send back success anyway
            # as we've already stored the data locally
            result = {
                "status": "partial_success", 
                "message": f"Data stored locally but not forwarded: {str(e)}",
                "data": data
            }
        
        return jsonify(result)
    except Exception as e:
        return jsonify({"status": "error", "message": str(e)}), 500

@app.route('/api/data/recent', methods=['GET'])
def get_recent_data():
    """Return recent data for visualization"""
    return jsonify({
        "status": "ok",
        "data": recent_data
    })

@socketio.on('connect')
def handle_connect():
    print('Client connected')

@socketio.on('disconnect')
def handle_disconnect():
    print('Client disconnected')

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5003))
    print(f"Starting Visualization service on port {port}...")
    socketio.run(app, host='0.0.0.0', port=port, debug=False)
