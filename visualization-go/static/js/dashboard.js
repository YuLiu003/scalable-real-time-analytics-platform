// Dashboard JavaScript

let temperatureChart = null;
let humidityChart = null;
let websocket = null;
let tenantId = 'tenant1'; // Default tenant ID
let realTimeUpdates = true;
let lastDataTimestamp = null;
let activityData = [];

// Initialize the dashboard when the document is loaded
document.addEventListener('DOMContentLoaded', function() {
    // Extract tenant ID from URL if available
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.has('tenant')) {
        tenantId = urlParams.get('tenant');
        document.getElementById('tenant-select').value = tenantId;
    }

    // Initialize charts
    initCharts();
    
    // Connect to WebSocket for real-time updates
    connectWebsocket();
    
    // Load initial data
    loadInitialData();
    
    // Set up event listeners
    document.getElementById('sendDataBtn').addEventListener('click', sendData);
    document.getElementById('checkStatusBtn').addEventListener('click', checkStatus);
    document.getElementById('changeTenantBtn').addEventListener('click', changeTenant);
    document.getElementById('refreshBtn').addEventListener('click', refreshData);
    
    // Set up real-time toggle
    document.getElementById('real-time-toggle').addEventListener('change', function(e) {
        realTimeUpdates = e.target.checked;
        if (realTimeUpdates && websocket === null) {
            connectWebsocket();
        }
    });
});

// Initialize charts
function initCharts() {
    const temperatureCtx = document.getElementById('temperatureChart').getContext('2d');
    temperatureChart = new Chart(temperatureCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'Temperature (°C)',
                data: [],
                borderColor: 'rgb(255, 99, 132)',
                backgroundColor: 'rgba(255, 99, 132, 0.1)',
                tension: 0.1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: false
                }
            },
            plugins: {
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            let label = context.dataset.label || '';
                            if (label) {
                                label += ': ';
                            }
                            if (context.parsed.y !== null) {
                                label += context.parsed.y.toFixed(1) + '°C';
                            }
                            return label;
                        }
                    }
                }
            }
        }
    });

    const humidityCtx = document.getElementById('humidityChart').getContext('2d');
    humidityChart = new Chart(humidityCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'Humidity (%)',
                data: [],
                borderColor: 'rgb(54, 162, 235)',
                backgroundColor: 'rgba(54, 162, 235, 0.1)',
                tension: 0.1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: false
                }
            },
            plugins: {
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            let label = context.dataset.label || '';
                            if (label) {
                                label += ': ';
                            }
                            if (context.parsed.y !== null) {
                                label += context.parsed.y.toFixed(1) + '%';
                            }
                            return label;
                        }
                    }
                }
            }
        }
    });
}

// Connect to WebSocket for real-time updates
function connectWebsocket() {
    // Don't connect if real-time updates are disabled
    if (!realTimeUpdates) {
        updateConnectionStatus('disabled');
        return;
    }
    
    // Check if browser supports WebSocket
    if (!window.WebSocket) {
        console.error('Your browser does not support WebSockets');
        updateConnectionStatus('error');
        return;
    }

    // Close existing connection if any
    if (websocket) {
        websocket.close();
    }

    // Create WebSocket connection
    const protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
    const wsUrl = `${protocol}${window.location.host}/ws?tenant=${tenantId}`;
    
    console.log('Connecting to WebSocket at:', wsUrl);
    
    try {
        websocket = new WebSocket(wsUrl);
        
        // Connection opened
        websocket.onopen = function(evt) {
            console.log('WebSocket connection established');
            updateConnectionStatus('online');
        };
        
        // Listen for messages
        websocket.onmessage = function(evt) {
            try {
                const data = JSON.parse(evt.data);
                console.log('WebSocket data received:', data);
                
                // Only process data for our tenant
                if (realTimeUpdates && (!data.tenant_id || data.tenant_id === tenantId)) {
                    updateCharts(data);
                    updateStats(data);
                    addToTimeline(data);
                }
            } catch (e) {
                console.error('Error processing WebSocket message:', e, evt.data);
            }
        };
        
        // Connection closed
        websocket.onclose = function(evt) {
            console.log('WebSocket connection closed, reason:', evt.reason);
            updateConnectionStatus('offline');
            
            // Store reference to avoid null checks
            websocket = null;
            
            // Try to reconnect after a delay if real-time updates are still enabled
            if (realTimeUpdates) {
                console.log('Attempting to reconnect in 5 seconds...');
                setTimeout(connectWebsocket, 5000);
            }
        };
        
        // Connection error
        websocket.onerror = function(evt) {
            console.error('WebSocket error:', evt);
            updateConnectionStatus('error');
        };
    } catch (e) {
        console.error('Error initializing WebSocket:', e);
        updateConnectionStatus('error');
    }
}

// Update connection status indicator
function updateConnectionStatus(status) {
    const statusElement = document.getElementById('ws-status');
    
    if (status === 'online') {
        statusElement.textContent = 'Online';
        statusElement.className = 'status-online';
    } else if (status === 'offline') {
        statusElement.textContent = 'Offline';
        statusElement.className = 'status-offline';
    } else if (status === 'error') {
        statusElement.textContent = 'Error';
        statusElement.className = 'status-offline';
    } else if (status === 'disabled') {
        statusElement.textContent = 'Disabled';
        statusElement.className = '';
    }
}

// Change tenant and reload data
function changeTenant() {
    tenantId = document.getElementById('tenant-select').value;
    
    // Update URL without reloading the page
    const url = new URL(window.location);
    url.searchParams.set('tenant', tenantId);
    window.history.pushState({}, '', url);
    
    // Reset charts and data
    resetData();
    
    // Load data for the new tenant
    loadInitialData();
    
    // Reconnect WebSocket with new tenant ID
    if (realTimeUpdates) {
        connectWebsocket();
    }
}

// Reset all data displays
function resetData() {
    // Clear charts
    temperatureChart.data.labels = [];
    temperatureChart.data.datasets[0].data = [];
    temperatureChart.update();
    
    humidityChart.data.labels = [];
    humidityChart.data.datasets[0].data = [];
    humidityChart.update();
    
    // Reset stats
    document.getElementById('avgTemp').textContent = '--';
    document.getElementById('avgHumidity').textContent = '--';
    document.getElementById('totalReadings').textContent = '0';
    document.getElementById('lastReading').textContent = '--';
    
    // Clear timeline
    document.getElementById('activity-timeline').innerHTML = '';
    activityData = [];
}

// Refresh data manually
function refreshData() {
    resetData();
    loadInitialData();
}

// Load initial data
function loadInitialData() {
    // Show loading indicator 
    const statusElement = document.getElementById('statusResults');
    if (statusElement) {
        statusElement.innerHTML = '<div class="loading">Loading data...</div>';
    }
    
    // Update charts to show loading
    updateLoadingState(true);
    
    // Make API request to get tenant data
    fetch(`/api/data/${tenantId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! Status: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            // Clear loading state
            updateLoadingState(false);
            
            console.log('Data loaded successfully:', data);
            
            // Accept both 'success' and 'ok' status values
            if ((data.status === 'success' || data.status === 'ok') && data.data && Array.isArray(data.data)) {
                if (data.data.length > 0) {
                    console.log("Data array for charts:", data.data); // Debug log
                    // Process historical data
                    const tempData = [];
                    const humidityData = [];
                    const labels = [];
                    
                    // Store for timeline
                    activityData = [];
                    
                    // Process data in reverse order (newest first)
                    data.data.forEach(item => {
                        if (item && item.timestamp) {
                            const date = new Date(item.timestamp);
                            labels.unshift(formatTime(date)); // Add to beginning for chronological order
                            
                            // Store for timeline
                            activityData.push(item);
                        }
                        
                        if (item && item.temperature !== null && item.temperature !== undefined) {
                            tempData.unshift(item.temperature); // Add to beginning for chronological order
                        }
                        
                        if (item && item.humidity !== null && item.humidity !== undefined) {
                            humidityData.unshift(item.humidity); // Add to beginning for chronological order
                        }
                    });
                    
                    // Update charts with historical data
                    temperatureChart.data.labels = labels;
                    temperatureChart.data.datasets[0].data = tempData;
                    temperatureChart.update();
                    
                    humidityChart.data.labels = labels;
                    humidityChart.data.datasets[0].data = humidityData;
                    humidityChart.update();
                    
                    // Update stats with the latest data
                    const latest = data.data[0];
                    if (latest) {
                        updateStats(latest);
                    }
                    
                    // Populate timeline
                    updateTimeline();
                    
                    // Store timestamp of the most recent data
                    if (data.data[0] && data.data[0].timestamp) {
                        lastDataTimestamp = new Date(data.data[0].timestamp);
                    }
                } else {
                    console.log('No data available yet');
                    // Show empty state in the timeline
                    document.getElementById('activity-timeline').innerHTML = '<p>No data available yet. Send some test data to get started.</p>';
                }
            } else {
                console.warn('Invalid data format received:', data);
                if (statusElement) {
                    statusElement.innerHTML = `<div class="warning">No data available or invalid format.</div>`;
                }
            }
            
            // Clear loading indicator
            if (statusElement) {
                statusElement.innerHTML = '';
            }
        })
        .catch(error => {
            // Clear loading state
            updateLoadingState(false);
            
            console.error('Error loading data:', error);
            // Show error in status results if available
            if (statusElement) {
                statusElement.innerHTML = `<div class="error">Failed to load data: ${error.message}</div>`;
            }
            
            // Try to reconnect after a delay
            setTimeout(loadInitialData, 10000);
        });
}

// Update charts to show loading state
function updateLoadingState(isLoading) {
    if (isLoading) {
        // Clear charts but keep structure
        if (temperatureChart) {
            temperatureChart.data.labels = [];
            temperatureChart.data.datasets[0].data = [];
            temperatureChart.update();
        }
        
        if (humidityChart) {
            humidityChart.data.labels = [];
            humidityChart.data.datasets[0].data = [];
            humidityChart.update();
        }
        
        // Clear stats
        document.getElementById('avgTemp').textContent = '--';
        document.getElementById('avgHumidity').textContent = '--';
        document.getElementById('totalReadings').textContent = '0';
        document.getElementById('lastReading').textContent = '--';
        
        // Show loading in timeline
        document.getElementById('activity-timeline').innerHTML = '<p>Loading data...</p>';
    }
}

// Send new data
function sendData() {
    const deviceId = document.getElementById('deviceId').value;
    const temperature = parseFloat(document.getElementById('temperature').value);
    const humidity = parseFloat(document.getElementById('humidity').value);
    const apiKey = document.getElementById('api-key').value;

    // Validate input
    if (!deviceId || isNaN(temperature) || isNaN(humidity)) {
        document.getElementById('response').innerHTML = `<div class="error">Please provide valid values for all fields</div>`;
        return;
    }

    if (!apiKey) {
        document.getElementById('response').innerHTML = `<div class="error">Error: Missing API key</div>`;
        return;
    }

    // Create data payload
    const data = {
        device_id: deviceId,
        temperature: temperature,
        humidity: humidity,
        tenant_id: tenantId,
        timestamp: new Date().toISOString()
    };

    // Send data to the API
    fetch('/api/data', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-API-Key': apiKey
        },
        body: JSON.stringify(data)
    })
    .then(response => response.json())
    .then(result => {
        const responseElement = document.getElementById('response');
        if (result.status === 'success' || result.status === 'ok' || result.status === 'partial_success') {
            responseElement.innerHTML = `<div class="success">Data sent successfully!</div>`;
            
            // If we're not getting real-time updates via WebSocket, manually update the UI
            if (!realTimeUpdates) {
                updateCharts(data);
                updateStats(data);
                addToTimeline(data);
            }
        } else {
            responseElement.innerHTML = `<div class="error">Error: ${result.message}</div>`;
        }
    })
    .catch(error => {
        console.error('Error sending data:', error);
        document.getElementById('response').innerHTML = `<div class="error">Error: ${error.message}</div>`;
    });
}

// Check service status
function checkStatus() {
    // Show loading indicator
    const statusElement = document.getElementById('statusResults');
    statusElement.innerHTML = '<div class="loading">Loading status...</div>';
    
    fetch('/api/status')
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! Status: ${response.status}`);
            }
            return response.json();
        })
        .then(status => {
            // Validate the structure of the response
            if (!status || typeof status !== 'object' || !status.services) {
                throw new Error('Invalid response structure: missing services object');
            }
            
            let statusHtml = '<pre>';
            
            for (const [service, info] of Object.entries(status.services)) {
                if (!info || typeof info !== 'object') {
                    // Skip invalid service entries
                    console.warn(`Invalid service info for ${service}:`, info);
                    continue;
                }
                
                const statusValue = info.status || 'unknown';
                const statusClass = statusValue === 'ok' || statusValue === 'healthy' ? 'success' : 'error';
                statusHtml += `<span class="${statusClass}">${service}</span>: ${statusValue}\n`;
                
                // Add additional message if available
                if (info.message) {
                    statusHtml += `  <small>${info.message}</small>\n`;
                }
            }
            
            statusHtml += '</pre>';
            statusElement.innerHTML = statusHtml;
        })
        .catch(error => {
            console.error('Error checking status:', error);
            statusElement.innerHTML = `<div class="error">Error: ${error.message}</div>`;
        });
}

// Update charts with new data
function updateCharts(data) {
    // Format timestamp
    let timeLabel = 'Unknown';
    let timestamp = null;
    
    if (data.timestamp) {
        timestamp = new Date(data.timestamp);
        timeLabel = formatTime(timestamp);
    } else {
        timestamp = new Date();
        timeLabel = formatTime(timestamp);
    }
    
    // Check if this data is newer than our last data point
    if (lastDataTimestamp && timestamp <= lastDataTimestamp) {
        // Skip processing older data
        return;
    }
    
    lastDataTimestamp = timestamp;
    
    // Add new data to temperature chart
    if (data.temperature !== null && data.temperature !== undefined) {
        temperatureChart.data.labels.push(timeLabel);
        temperatureChart.data.datasets[0].data.push(data.temperature);
        
        // Limit chart to last 20 data points for better visibility
        if (temperatureChart.data.labels.length > 20) {
            temperatureChart.data.labels.shift();
            temperatureChart.data.datasets[0].data.shift();
        }
        
        temperatureChart.update();
    }
    
    // Add new data to humidity chart
    if (data.humidity !== null && data.humidity !== undefined) {
        humidityChart.data.labels.push(timeLabel);
        humidityChart.data.datasets[0].data.push(data.humidity);
        
        // Limit chart to last 20 data points for better visibility
        if (humidityChart.data.labels.length > 20) {
            humidityChart.data.labels.shift();
            humidityChart.data.datasets[0].data.shift();
        }
        
        humidityChart.update();
    }
}

// Update statistics display
function updateStats(data) {
    // Update total readings
    const totalElement = document.getElementById('totalReadings');
    const currentTotal = parseInt(totalElement.textContent) || 0;
    totalElement.textContent = currentTotal + 1;
    
    // Update last reading time
    if (data.timestamp) {
        document.getElementById('lastReading').textContent = formatTime(new Date(data.timestamp));
    }
    
    // Update averages if available
    if (data.temperature_stats && data.temperature_stats.avg) {
        document.getElementById('avgTemp').textContent = data.temperature_stats.avg.toFixed(1);
    } else if (data.temperature !== null && data.temperature !== undefined) {
        // If stats are not available, just use the current temperature as a fallback
        const temp = document.getElementById('avgTemp');
        if (temp.textContent === '--') {
            temp.textContent = data.temperature.toFixed(1);
        } else {
            // Simple running average
            const currentAvg = parseFloat(temp.textContent);
            const newAvg = ((currentAvg * currentTotal) + data.temperature) / (currentTotal + 1);
            temp.textContent = newAvg.toFixed(1);
        }
    }
    
    if (data.humidity_stats && data.humidity_stats.avg) {
        document.getElementById('avgHumidity').textContent = data.humidity_stats.avg.toFixed(1);
    } else if (data.humidity !== null && data.humidity !== undefined) {
        // If stats are not available, just use the current humidity as a fallback
        const humidity = document.getElementById('avgHumidity');
        if (humidity.textContent === '--') {
            humidity.textContent = data.humidity.toFixed(1);
        } else {
            // Simple running average
            const currentAvg = parseFloat(humidity.textContent);
            const newAvg = ((currentAvg * currentTotal) + data.humidity) / (currentTotal + 1);
            humidity.textContent = newAvg.toFixed(1);
        }
    }
}

// Add a data point to the activity timeline
function addToTimeline(data) {
    // Add to activity data array (limit to 20 most recent entries)
    activityData.unshift(data);
    if (activityData.length > 20) {
        activityData.pop();
    }
    
    // Update the timeline display
    updateTimeline();
}

// Update the timeline with the current activity data
function updateTimeline() {
    const timeline = document.getElementById('activity-timeline');
    timeline.innerHTML = '';
    
    activityData.forEach(data => {
        const timelineItem = document.createElement('div');
        timelineItem.className = 'timeline-item';
        
        // Add anomaly class if applicable
        if (data.is_anomaly) {
            timelineItem.classList.add('anomaly');
        }
        
        const timeString = data.timestamp 
            ? formatTime(new Date(data.timestamp))
            : formatTime(new Date());
            
        const deviceId = data.device_id || 'unknown';
        
        let details = '';
        if (data.temperature !== null && data.temperature !== undefined) {
            details += `Temperature: ${data.temperature.toFixed(1)}°C `;
        }
        if (data.humidity !== null && data.humidity !== undefined) {
            details += `Humidity: ${data.humidity.toFixed(1)}% `;
        }
        
        timelineItem.innerHTML = `
            <div class="timeline-content">
                <div class="timeline-time">${timeString}</div>
                <div class="timeline-title">Device: ${deviceId}</div>
                <div class="timeline-details">${details}</div>
            </div>
        `;
        
        timeline.appendChild(timelineItem);
    });
    
    // Show placeholder if no data
    if (activityData.length === 0) {
        timeline.innerHTML = '<p>No activity data available</p>';
    }
}

// Format time as HH:MM:SS
function formatTime(date) {
    return date.toLocaleTimeString();
}

// Format date as YYYY-MM-DD
function formatDate(date) {
    return date.toLocaleDateString();
}

// Format as date and time
function formatDateTime(date) {
    return `${formatDate(date)} ${formatTime(date)}`;
} 