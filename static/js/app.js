const form = document.getElementById('urlForm');
const input = document.getElementById('igUrl');
const errorEl = document.getElementById('errorMsg');
const loadEl = document.getElementById('loadingState');

function showError(msg) {
    errorEl.textContent = msg;
    errorEl.hidden = false;
    loadEl.hidden = true;
    form.hidden = false;
    input.focus();
}

function showLoading() {
    errorEl.hidden = true;
    loadEl.hidden = false;
    form.hidden = true;
}

form.addEventListener('submit', async function (e) {
    e.preventDefault();
    const url = input.value.trim();
    if (!url) return;

    showLoading();

    try {
        // Wait for the server (covers slow video downloads), then navigate.
        // On success the player is served from cache; on error the server
        // renders the styled error page — spinner disappears either way.
        await fetch('/' + url, { redirect: 'follow' });
        window.location.href = '/' + url;
    } catch (err) {
        showError('Network error. Please check your connection and try again.');
    }
});

document.getElementById('originHint').textContent = window.location.origin + '/';

window.addEventListener('pageshow', function (e) {
    if (e.persisted) {
        // Reset state when restoring from bfcache (e.g. clicking the browser Back button)
        errorEl.hidden = true;
        loadEl.hidden = true;
        form.hidden = false;
    }
});
