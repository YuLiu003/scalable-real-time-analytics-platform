// Admin UI JavaScript
document.addEventListener('DOMContentLoaded', function() {
    // Initialize Bootstrap components
    const createTenantModal = new bootstrap.Modal(document.getElementById('createTenantModal'));
    const apiKeyModal = new bootstrap.Modal(document.getElementById('apiKeyModal'));
    
    // Get UI elements
    const createTenantBtn = document.getElementById('createTenantBtn');
    const submitCreateTenantBtn = document.getElementById('submitCreateTenant');
    const refreshBtn = document.getElementById('refreshBtn');
    const checkSystemBtn = document.getElementById('checkSystemBtn');
    const tenantsTableBody = document.getElementById('tenantsTableBody');
    const copyApiKeyBtn = document.getElementById('copyApiKey');
    const regenerateApiKeyBtn = document.getElementById('regenerateApiKey');
    
    // Initialize event listeners
    createTenantBtn.addEventListener('click', () => createTenantModal.show());
    submitCreateTenantBtn.addEventListener('click', createTenant);
    refreshBtn.addEventListener('click', loadTenants);
    checkSystemBtn.addEventListener('click', checkSystemStatus);
    copyApiKeyBtn.addEventListener('click', copyApiKey);
    regenerateApiKeyBtn.addEventListener('click', regenerateApiKey);
    
    // Load initial data
    loadTenants();
    checkSystemStatus();
    
    // Set refresh interval
    setInterval(checkSystemStatus, 60000); // Check system status every minute
    
    // Functions
    function loadTenants() {
        fetch('/api/tenants')
            .then(response => response.json())
            .then(data => {
                // Clear table
                tenantsTableBody.innerHTML = '';
                
                // Update active tenant count
                const activeTenants = document.getElementById('activeTenants');
                activeTenants.textContent = data.tenants ? data.tenants.length : 0;
                
                // Add tenant rows
                if (data.tenants && data.tenants.length > 0) {
                    data.tenants.forEach(tenant => {
                        const row = document.createElement('tr');
                        
                        // Format date
                        const createdAt = new Date(tenant.created_at);
                        const formattedDate = createdAt.toLocaleDateString();
                        
                        row.innerHTML = `
                            <td>${tenant.tenant_id}</td>
                            <td>${tenant.name}</td>
                            <td><span class="badge bg-info">${tenant.plan || 'basic'}</span></td>
                            <td>${formattedDate}</td>
                            <td>
                                <button class="btn btn-sm btn-outline-primary view-api-key" data-tenant-id="${tenant.tenant_id}">API Key</button>
                                <button class="btn btn-sm btn-outline-danger delete-tenant" data-tenant-id="${tenant.tenant_id}">Delete</button>
                            </td>
                        `;
                        
                        tenantsTableBody.appendChild(row);
                    });
                    
                    // Add event listeners to the new buttons
                    document.querySelectorAll('.view-api-key').forEach(btn => {
                        btn.addEventListener('click', viewApiKey);
                    });
                    
                    document.querySelectorAll('.delete-tenant').forEach(btn => {
                        btn.addEventListener('click', deleteTenant);
                    });
                } else {
                    const row = document.createElement('tr');
                    row.innerHTML = '<td colspan="5" class="text-center">No tenants found</td>';
                    tenantsTableBody.appendChild(row);
                }
            })
            .catch(error => {
                console.error('Error loading tenants:', error);
                showNotification('Error loading tenants', 'danger');
            });
    }
    
    function createTenant() {
        const name = document.getElementById('tenantName').value;
        const plan = document.getElementById('tenantPlan').value;
        
        if (!name) {
            showNotification('Tenant name is required', 'warning');
            return;
        }
        
        fetch('/api/tenants', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                name: name,
                plan: plan
            })
        })
        .then(response => response.json())
        .then(data => {
            createTenantModal.hide();
            if (data.tenant_id) {
                showNotification(`Tenant ${name} created successfully`, 'success');
                loadTenants();
                
                // Reset form
                document.getElementById('createTenantForm').reset();
            } else {
                showNotification(`Error: ${data.message || 'Unknown error'}`, 'danger');
            }
        })
        .catch(error => {
            console.error('Error creating tenant:', error);
            showNotification('Error creating tenant', 'danger');
        });
    }
    
    function deleteTenant(event) {
        const tenantId = event.target.getAttribute('data-tenant-id');
        
        if (confirm(`Are you sure you want to delete tenant ${tenantId}?`)) {
            fetch(`/api/tenants/${tenantId}`, {
                method: 'DELETE'
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    showNotification(`Tenant ${tenantId} deleted successfully`, 'success');
                    loadTenants();
                } else {
                    showNotification(`Error: ${data.message || 'Unknown error'}`, 'danger');
                }
            })
            .catch(error => {
                console.error('Error deleting tenant:', error);
                showNotification('Error deleting tenant', 'danger');
            });
        }
    }
    
    function viewApiKey(event) {
        const tenantId = event.target.getAttribute('data-tenant-id');
        document.getElementById('apiKeyTenantId').textContent = tenantId;
        
        // In a real app, you'd fetch this from the server
        // For demo, generate a mock API key
        const mockApiKey = `tenant_${tenantId}_${generateRandomString(32)}`;
        document.getElementById('apiKeyValue').value = mockApiKey;
        
        apiKeyModal.show();
    }
    
    function copyApiKey() {
        const apiKey = document.getElementById('apiKeyValue');
        apiKey.select();
        document.execCommand('copy');
        showNotification('API key copied to clipboard', 'success');
    }
    
    function regenerateApiKey() {
        const tenantId = document.getElementById('apiKeyTenantId').textContent;
        
        // In a real app, you'd make a server request to regenerate
        // For demo, just generate a new mock key
        const newApiKey = `tenant_${tenantId}_${generateRandomString(32)}`;
        document.getElementById('apiKeyValue').value = newApiKey;
        
        showNotification('API key regenerated', 'success');
    }
    
    function checkSystemStatus() {
        fetch('/api/status')
            .then(response => response.json())
            .then(data => {
                // Update status indicators
                updateStatusBadge('apiStatus', data.services.api?.status || 'unknown');
                updateStatusBadge('kafkaStatus', data.services.kafka?.status || 'unknown');
                updateStatusBadge('storageStatus', data.services.storage?.status || 'unknown');
                
                // Update active tenants if available
                if (data.resources && data.resources.active_tenants !== undefined) {
                    document.getElementById('activeTenants').textContent = data.resources.active_tenants;
                }
            })
            .catch(error => {
                console.error('Error checking system status:', error);
                
                // Set all to unknown on error
                updateStatusBadge('apiStatus', 'unknown');
                updateStatusBadge('kafkaStatus', 'unknown');
                updateStatusBadge('storageStatus', 'unknown');
            });
    }
    
    function updateStatusBadge(elementId, status) {
        const element = document.getElementById(elementId);
        
        // Remove existing status classes
        element.classList.remove('bg-success', 'bg-warning', 'bg-danger', 'bg-secondary');
        
        // Add appropriate class based on status
        switch (status) {
            case 'operational':
                element.classList.add('bg-success');
                element.textContent = 'Online';
                break;
            case 'degraded':
                element.classList.add('bg-warning');
                element.textContent = 'Degraded';
                break;
            case 'error':
                element.classList.add('bg-danger');
                element.textContent = 'Offline';
                break;
            default:
                element.classList.add('bg-secondary');
                element.textContent = 'Unknown';
        }
    }
    
    function showNotification(message, type) {
        // Create notification element
        const notification = document.createElement('div');
        notification.className = `toast align-items-center text-white bg-${type}`;
        notification.setAttribute('role', 'alert');
        notification.setAttribute('aria-live', 'assertive');
        notification.setAttribute('aria-atomic', 'true');
        
        notification.innerHTML = `
            <div class="d-flex">
                <div class="toast-body">
                    ${message}
                </div>
                <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
            </div>
        `;
        
        // Add to container or create one if it doesn't exist
        let container = document.querySelector('.toast-container');
        if (!container) {
            container = document.createElement('div');
            container.className = 'toast-container position-fixed top-0 end-0 p-3';
            document.body.appendChild(container);
        }
        
        container.appendChild(notification);
        
        // Initialize and show toast
        const toast = new bootstrap.Toast(notification, { autohide: true, delay: 3000 });
        toast.show();
        
        // Remove from DOM after hiding
        notification.addEventListener('hidden.bs.toast', function() {
            notification.remove();
        });
    }
    
    function generateRandomString(length) {
        const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
        let result = '';
        for (let i = 0; i < length; i++) {
            result += characters.charAt(Math.floor(Math.random() * characters.length));
        }
        return result;
    }
}); 