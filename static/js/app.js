const form = document.getElementById('urlForm');
const input = document.getElementById('videoUrl');
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
    let url = input.value.trim();
    if (!url) return;

    try {
        let parseUrl = url;
        if (!parseUrl.startsWith('http://') && !parseUrl.startsWith('https://')) {
            parseUrl = 'https://' + parseUrl;
        }
        const parsedUrl = new URL(parseUrl);
        const host = parsedUrl.hostname.toLowerCase();
        const isFacebook = host.includes('facebook.com') || host.includes('fb.watch');

        if (!isFacebook) {
            parsedUrl.search = '';
        }
        parsedUrl.hash = '';
        url = parsedUrl.toString();
    } catch (e) {
        // Let the backend validate if it's not a parsable URL
    }

    showLoading();

    try {
        await fetch('/' + url, { redirect: 'follow' });
        window.location.href = '/' + url;
    } catch (err) {
        showError('Network error. Please check your connection and try again.');
    }
});

document.getElementById('originHint').textContent = window.location.origin + '/';

window.addEventListener('pageshow', function (e) {
    if (e.persisted) {
        // Restoring from bfcache (e.g. clicking the browser Back button)
        errorEl.hidden = true;
        loadEl.hidden = true;
        form.hidden = false;
    }
});
