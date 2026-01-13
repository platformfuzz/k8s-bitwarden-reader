// WebSocket connection management
let ws = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;
let reconnectTimeout = null;

const secretVisibilityState = new Map();

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    updateConnectionStatus('connecting', 'Connecting...');

    ws = new WebSocket(wsUrl);

    ws.onopen = function() {
        reconnectAttempts = 0;
        updateConnectionStatus('connected', 'Connected');
    };

    ws.onclose = function() {
        updateConnectionStatus('disconnected', 'Disconnected');
        attemptReconnect();
    };

    ws.onerror = function(error) {
        console.error('WebSocket error:', error);
        updateConnectionStatus('disconnected', 'Connection Error');
    };

    ws.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            updateSecrets(data);
        } catch (error) {
            console.error('Error parsing WebSocket message:', error);
        }
    };
}

function updateConnectionStatus(status, message) {
    const statusElement = document.getElementById('ws-status');
    if (statusElement) {
        statusElement.textContent = message;
        statusElement.className = status;
    }
}

function attemptReconnect() {
    if (reconnectAttempts < maxReconnectAttempts) {
        reconnectAttempts++;
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempts - 1), 30000);
        reconnectTimeout = setTimeout(() => {
            connectWebSocket();
        }, delay);
    } else {
        updateConnectionStatus('disconnected', 'Connection Lost - Please refresh');
    }
}

// Update secrets display with new data
function updateSecrets(data) {
    if (!data.secrets) return;

    const container = document.getElementById('secrets-container');
    if (!container) return;

    // Update total found count
    const h2 = document.querySelector('.secrets-section h2');
    if (h2) {
        h2.textContent = `Secrets (${data.totalFound} found)`;
    }

    // Update each secret card
    data.secrets.forEach(secret => {
        const card = document.querySelector(`[data-secret-name="${secret.name}"]`);
        if (!card) return;

        // Update found status
        const statusBadge = card.querySelector('.status-badge');
        if (statusBadge) {
            if (secret.found) {
                statusBadge.textContent = 'Found';
                statusBadge.className = 'status-badge status-found';
            } else {
                statusBadge.textContent = 'Not Found';
                statusBadge.className = 'status-badge status-not-found';
            }
        }

        // Update error message
        const errorDiv = card.querySelector('.error-message');
        if (secret.error) {
            if (!errorDiv) {
                const errorElement = document.createElement('div');
                errorElement.className = 'error-message';
                errorElement.innerHTML = `<strong>Error:</strong> ${escapeHtml(secret.error)}`;
                card.insertBefore(errorElement, card.firstChild.nextSibling);
            } else {
                errorDiv.innerHTML = `<strong>Error:</strong> ${escapeHtml(secret.error)}`;
            }
        } else if (errorDiv) {
            errorDiv.remove();
        }

        // Update sync info
        if (secret.found && secret.syncInfo) {
            updateSyncInfo(card, secret.syncInfo);
        }

        // Update secret keys
        if (secret.found && secret.keys) {
            updateSecretKeys(card, secret.name, secret.keys);
        }
    });
}

function updateSyncInfo(card, syncInfo) {
    const syncInfoDiv = card.querySelector('.sync-info');
    if (!syncInfoDiv) return;

    // Update CRD Found
    const crdFound = syncInfoDiv.querySelector('.sync-item:has(strong:contains("CRD Found"))');
    if (syncInfo.crdFound !== undefined) {
        // This would need more sophisticated DOM manipulation
        // For now, we'll rely on server-side rendering for initial load
    }
}

function updateSecretKeys(card, secretName, keys) {
    const keysList = card.querySelector(`#keys-${secretName}`);
    if (!keysList) return;

    const existingItems = keysList.querySelectorAll('.key-item');
    const keysArray = Object.entries(keys);

    if (existingItems.length !== keysArray.length) {
        const isVisible = secretVisibilityState.get(secretName) || false;
        keysList.innerHTML = '';
        keysArray.forEach(([key, value]) => {
            const keyItem = document.createElement('div');
            keyItem.className = 'key-item';
            keyItem.innerHTML = `
                <strong>${escapeHtml(key)}:</strong>
                <span class="secret-value" data-secret="${escapeHtml(secretName)}"
                      data-key="${escapeHtml(key)}"
                      style="display: ${isVisible ? 'inline' : 'none'};">${escapeHtml(value)}</span>
                <span class="secret-placeholder" data-secret="${escapeHtml(secretName)}"
                      data-key="${escapeHtml(key)}"
                      style="display: ${isVisible ? 'none' : 'inline'};">••••••••</span>
            `;
            keysList.appendChild(keyItem);
        });
        const toggleBtn = card.querySelector('.btn-toggle');
        if (toggleBtn) {
            toggleBtn.textContent = isVisible ? 'Hide Values' : 'Show Values';
        }
    } else {
        keysArray.forEach(([key, value], index) => {
            const keyItem = existingItems[index];
            if (keyItem) {
                const valueSpan = keyItem.querySelector('.secret-value');
                if (valueSpan && valueSpan.textContent !== value) {
                    valueSpan.textContent = value;
                }
            }
        });
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

window.toggleSecretValues = function(secretName) {
    console.log('toggleSecretValues called with:', secretName);

    const card = document.querySelector(`[data-secret-name="${secretName}"]`);
    if (!card) {
        console.error('Card not found for secret:', secretName);
        return;
    }

    const values = card.querySelectorAll(`.secret-value[data-secret="${secretName}"]`);
    const placeholders = card.querySelectorAll(`.secret-placeholder[data-secret="${secretName}"]`);
    const toggleBtn = card.querySelector('.btn-toggle');

    console.log('Found elements:', {
        values: values.length,
        placeholders: placeholders.length,
        toggleBtn: !!toggleBtn
    });

    if (values.length === 0) {
        console.error('No secret values found for:', secretName);
        return;
    }

    const firstValue = values[0];
    const computedStyle = window.getComputedStyle(firstValue);
    const isVisible = computedStyle.display !== 'none';
    const newVisibilityState = !isVisible;

    values.forEach(el => {
        el.style.display = newVisibilityState ? 'inline' : 'none';
    });

    placeholders.forEach(el => {
        el.style.display = newVisibilityState ? 'none' : 'inline';
    });

    if (toggleBtn) {
        toggleBtn.textContent = newVisibilityState ? 'Hide Values' : 'Show Values';
    }

    secretVisibilityState.set(secretName, newVisibilityState);

    console.log('Toggle complete. New state:', newVisibilityState ? 'visible' : 'hidden');
    console.log('Stored state for', secretName, ':', newVisibilityState);
}

// Trigger sync functionality
async function triggerSync() {
    const btn = document.getElementById('trigger-sync-btn');
    const statusSpan = document.getElementById('sync-status');

    if (!btn || !statusSpan) return;

    btn.disabled = true;
    statusSpan.textContent = 'Triggering sync...';
    statusSpan.className = '';

    try {
        const response = await fetch('/api/v1/trigger-sync', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({})
        });

        const data = await response.json();

        if (response.ok) {
            statusSpan.textContent = 'Sync triggered successfully';
            statusSpan.className = 'success';

            // Poll for sync completion
            pollSyncStatus();
        } else {
            statusSpan.textContent = `Error: ${data.error || 'Unknown error'}`;
            statusSpan.className = 'error';
        }
    } catch (error) {
        statusSpan.textContent = `Error: ${error.message}`;
        statusSpan.className = 'error';
    } finally {
        btn.disabled = false;

        // Clear status after 5 seconds
        setTimeout(() => {
            statusSpan.textContent = '';
            statusSpan.className = '';
        }, 5000);
    }
}

// Poll for sync completion
async function pollSyncStatus() {
    const maxPolls = 30; // Poll for up to 30 times
    let pollCount = 0;

    const pollInterval = setInterval(async () => {
        pollCount++;

        try {
            const response = await fetch('/api/v1/secrets');
            const data = await response.json();

            // Check if sync is complete (this is a simplified check)
            // In a real implementation, you'd check the sync status more carefully

            if (pollCount >= maxPolls) {
                clearInterval(pollInterval);
            }
        } catch (error) {
            console.error('Error polling sync status:', error);
            clearInterval(pollInterval);
        }
    }, 2000); // Poll every 2 seconds
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    // Connect WebSocket
    connectWebSocket();

    // Setup trigger sync button
    const triggerBtn = document.getElementById('trigger-sync-btn');
    if (triggerBtn) {
        triggerBtn.addEventListener('click', triggerSync);
    }

    // Setup toggle buttons for each secret
    document.querySelectorAll('.btn-toggle').forEach(btn => {
        btn.addEventListener('click', function() {
            const secretCard = this.closest('.secret-card');
            if (secretCard) {
                const secretName = secretCard.getAttribute('data-secret-name');
                toggleSecretValues(secretName);
            }
        });
    });
});

// Cleanup on page unload
window.addEventListener('beforeunload', function() {
    if (ws) {
        ws.close();
    }
    if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
    }
});
