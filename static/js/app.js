document.getElementById('urlForm').addEventListener('submit', function (e) {
    e.preventDefault();
    const url = document.getElementById('igUrl').value.trim();
    if (url) {
        window.location.href = '/' + url;
    }
});

document.getElementById('originHint').textContent = window.location.origin + '/';
