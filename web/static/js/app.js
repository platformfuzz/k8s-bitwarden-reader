// WebSocket connection management
let ws = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;
let reconnectTimeout = null;

// Track secrets whose visibility is controlled by user toggle
const manuallyControlledSecrets = new Set();

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws`;

  updateConnectionStatus('connecting', 'Connecting...');

  ws = new WebSocket(wsUrl);

  ws.onopen = function () {
    reconnectAttempts = 0;
    updateConnectionStatus('connected', 'Connected');
  };

  ws.onclose = function () {
    updateConnectionStatus('disconnected', 'Disconnected');
    attemptReconnect();
  };

  ws.onerror = function (error) {
    console.error('WebSocket error:', error);
    updateConnectionStatus('disconnected', 'Connection Error');
  };

  ws.onmessage = function (event) {
    try {
      const data = JSON.parse(event.data);
      if (data.secrets) {
        data.secrets = data.secrets.filter(s => !manuallyControlledSecrets.has(s.name));
        if (data.secrets.length === 0) {
          return;
        }
      }
      if (data.secrets && data.secrets.length > 0) {
        updateSecrets(data);
      }
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
  if (!data.secrets || data.secrets.length === 0) return;

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
  // Sync info is primarily rendered server-side
  // This function is a placeholder for future dynamic updates
}

function updateSecretKeys(card, secretName, keys) {
  const keysList = card.querySelector(`#keys-${secretName}`);
  if (!keysList) return;

  const existingItems = keysList.querySelectorAll('.key-item');
  const keysArray = Object.entries(keys);

  // Preserve current visibility state when rebuilding
  let currentVisibility = false;
  if (existingItems.length > 0) {
    const firstDisplay = existingItems[0].querySelector('.secret-display');
    if (firstDisplay) {
      currentVisibility = firstDisplay.getAttribute('data-hidden') !== 'true';
    }
  }

  if (existingItems.length !== keysArray.length) {
    keysList.innerHTML = '';
    keysArray.forEach(([key, value]) => {
      const keyItem = document.createElement('div');
      keyItem.className = 'key-item';
      const hiddenAttr = currentVisibility ? 'false' : 'true';
      keyItem.innerHTML = `
                <strong>${escapeHtml(key)}:</strong>
                <span class="secret-display" data-secret="${escapeHtml(secretName)}"
                      data-key="${escapeHtml(key)}" data-value="${escapeHtml(value)}"
                      data-hidden="${hiddenAttr}">
                  <span class="secret-actual-value">${escapeHtml(value)}</span>
                  <span class="secret-masked-value">••••••••</span>
                </span>
            `;
      keysList.appendChild(keyItem);
    });
    const toggleBtn = card.querySelector('.btn-toggle');
    if (toggleBtn) {
      toggleBtn.textContent = currentVisibility ? 'Hide Values' : 'Show Values';
    }
  } else {
    keysArray.forEach(([key, value], index) => {
      const keyItem = existingItems[index];
      if (keyItem) {
        const displayEl = keyItem.querySelector('.secret-display');
        if (displayEl) {
          const currentValue = displayEl.getAttribute('data-value');
          const actualValueSpan = displayEl.querySelector('.secret-actual-value');

          if (currentValue !== escapeHtml(value)) {
            displayEl.setAttribute('data-value', escapeHtml(value));
            if (actualValueSpan) {
              actualValueSpan.textContent = escapeHtml(value);
            }
          }
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

window.toggleSecretValues = function (secretName) {
  manuallyControlledSecrets.add(secretName);

  const card = document.querySelector(`[data-secret-name="${secretName}"]`);
  if (!card) return;

  const keysList = card.querySelector(`#keys-${secretName}`);
  if (!keysList) return;

  const displays = keysList.querySelectorAll('.secret-display');
  if (displays.length === 0) return;

  const toggleBtn = card.querySelector('.btn-toggle');
  const firstDisplay = displays[0];
  const isHidden = firstDisplay.getAttribute('data-hidden') === 'true';
  const shouldShow = isHidden;

  displays.forEach((el) => {
    const actualSpan = el.querySelector('.secret-actual-value');
    const maskedSpan = el.querySelector('.secret-masked-value');

    if (shouldShow) {
      el.setAttribute('data-hidden', 'false');
      if (actualSpan) {
        actualSpan.style.display = 'inline';
        actualSpan.style.visibility = 'visible';
      }
      if (maskedSpan) {
        maskedSpan.style.display = 'none';
        maskedSpan.style.visibility = 'hidden';
      }
    } else {
      el.setAttribute('data-hidden', 'true');
      if (actualSpan) {
        actualSpan.style.display = 'none';
        actualSpan.style.visibility = 'hidden';
      }
      if (maskedSpan) {
        maskedSpan.style.display = 'inline';
        maskedSpan.style.visibility = 'visible';
      }
    }
  });

  if (toggleBtn) {
    toggleBtn.textContent = shouldShow ? 'Hide Values' : 'Show Values';
  }
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
  const maxPolls = 30;
  let pollCount = 0;

  const pollInterval = setInterval(async () => {
    pollCount++;

    try {
      const response = await fetch('/api/v1/secrets');
      await response.json();

      if (pollCount >= maxPolls) {
        clearInterval(pollInterval);
      }
    } catch (error) {
      console.error('Error polling sync status:', error);
      clearInterval(pollInterval);
    }
  }, 2000);
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', function () {
  // Connect WebSocket
  connectWebSocket();

  // Setup trigger sync button
  const triggerBtn = document.getElementById('trigger-sync-btn');
  if (triggerBtn) {
    triggerBtn.addEventListener('click', triggerSync);
  }
});

// Cleanup on page unload
window.addEventListener('beforeunload', function () {
  if (ws) {
    ws.close();
  }
  if (reconnectTimeout) {
    clearTimeout(reconnectTimeout);
  }
});
