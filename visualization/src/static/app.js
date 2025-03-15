const app = (() => {
    const metricsContainer = document.getElementById('metrics');
    const alertsContainer = document.getElementById('alerts');

    const fetchMetrics = async () => {
        try {
            const response = await fetch('/api/metrics');
            const data = await response.json();
            renderMetrics(data);
        } catch (error) {
            console.error('Error fetching metrics:', error);
        }
    };

    const renderMetrics = (data) => {
        metricsContainer.innerHTML = '';
        data.forEach(metric => {
            const metricElement = document.createElement('div');
            metricElement.className = 'metric';
            metricElement.innerHTML = `<strong>${metric.name}:</strong> ${metric.value}`;
            metricsContainer.appendChild(metricElement);
        });
    };

    const fetchAlerts = async () => {
        try {
            const response = await fetch('/api/alerts');
            const data = await response.json();
            renderAlerts(data);
        } catch (error) {
            console.error('Error fetching alerts:', error);
        }
    };

    const renderAlerts = (data) => {
        alertsContainer.innerHTML = '';
        data.forEach(alert => {
            const alertElement = document.createElement('div');
            alertElement.className = 'alert';
            alertElement.innerHTML = `<strong>Alert:</strong> ${alert.message}`;
            alertsContainer.appendChild(alertElement);
        });
    };

    const init = () => {
        fetchMetrics();
        fetchAlerts();
        setInterval(fetchMetrics, 5000); // Refresh metrics every 5 seconds
        setInterval(fetchAlerts, 10000); // Refresh alerts every 10 seconds
    };

    return {
        init
    };
})();

document.addEventListener('DOMContentLoaded', app.init);