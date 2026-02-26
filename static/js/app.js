document.getElementById('urlForm').addEventListener('submit', function (e) {
    e.preventDefault();
    const url = document.getElementById('igUrl').value.trim();
    if (url) {
        window.location.href = '/' + url;
    }
});

document.getElementById('originHint').textContent = window.location.origin + '/';

// Cookie grabbing functionality
const cookieBtn = document.getElementById('cookieBtn');
const cookieStatus = document.getElementById('cookieStatus');
const cookieIndicator = document.getElementById('cookieIndicator');

function updateIndicator(hasCookie, savedAt) {
    const text = cookieIndicator.querySelector('.indicator-text');

    if (hasCookie) {
        cookieIndicator.className = 'cookie-indicator has-cookie';
        let statusText = 'Cookie stored';
        if (savedAt) {
            const date = new Date(savedAt * 1000);
            const now = new Date();
            const diffMs = now - date;
            const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

            if (diffDays === 0) {
                statusText += ' (saved today)';
            } else if (diffDays === 1) {
                statusText += ' (saved yesterday)';
            } else {
                statusText += ` (saved ${diffDays} days ago)`;
            }
        }
        text.textContent = statusText;
    } else {
        cookieIndicator.className = 'cookie-indicator no-cookie';
        text.textContent = 'No cookie stored';
    }
}

// Check if cookie is already set
fetch('/api/cookie/status')
    .then(r => r.json())
    .then(data => {
        updateIndicator(data.hasCookie, data.savedAt);
    })
    .catch(() => {
        cookieIndicator.className = 'cookie-indicator no-cookie';
        cookieIndicator.querySelector('.indicator-text').textContent = 'Could not check status';
    });

cookieBtn.addEventListener('click', function () {
    // Toggle instructions visibility
    if (cookieStatus.classList.contains('instructions')) {
        cookieStatus.innerHTML = '';
        cookieStatus.className = 'cookie-status';
        return;
    }

    // Show instructions
    cookieStatus.innerHTML = `
        <div class="instructions-header">
            <strong>🔐 Get your Instagram session cookie</strong>
        </div>
        <ol class="instructions-list">
            <li>Open <a href="https://www.instagram.com/" target="_blank" rel="noopener">instagram.com</a> in a new tab and log in</li>
            <li>Press <kbd>F12</kbd> to open Developer Tools</li>
            <li>Click the <strong>Application</strong> tab (or <strong>Storage</strong> in Firefox)</li>
            <li>In the left sidebar, expand <strong>Cookies</strong> → click <strong>https://www.instagram.com</strong></li>
            <li>Find the row named <code>sessionid</code> and copy its <strong>Value</strong> (double-click to select)</li>
            <li>Paste it below and click Save</li>
        </ol>
        <div class="cookie-input-group">
            <input type="text" id="sessionIdInput" placeholder="Paste your sessionid here…" spellcheck="false">
            <button type="button" id="saveCookieBtn">Save</button>
        </div>
        <p class="instructions-note">⚠️ Keep this private — it grants access to your Instagram account.</p>
    `;
    cookieStatus.className = 'cookie-status instructions';

    // Attach event listener to the save button
    document.getElementById('saveCookieBtn').addEventListener('click', saveCookie);
});

function saveCookie() {
    const sessionId = document.getElementById('sessionIdInput').value.trim();
    if (!sessionId) {
        alert('Please paste the sessionid cookie value');
        return;
    }

    fetch('/api/cookie', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sessionid: sessionId })
    })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                cookieStatus.textContent = '✓ Cookie saved successfully!';
                cookieStatus.className = 'cookie-status success';
                updateIndicator(true, Math.floor(Date.now() / 1000));
            } else {
                cookieStatus.textContent = '✗ Failed to save cookie: ' + data.error;
                cookieStatus.className = 'cookie-status error';
            }
        })
        .catch(err => {
            cookieStatus.textContent = '✗ Error: ' + err.message;
            cookieStatus.className = 'cookie-status error';
        });
}
