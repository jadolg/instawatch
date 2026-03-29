const video = document.getElementById("videoPlayer");

document.addEventListener("keydown", function (e) {
    switch (e.code) {
        case "Space":
            e.preventDefault();
            video.paused ? video.play() : video.pause();
            break;
        case "ArrowLeft":
            e.preventDefault();
            video.currentTime = Math.max(0, video.currentTime - 5);
            break;
        case "ArrowRight":
            e.preventDefault();
            video.currentTime = Math.min(video.duration, video.currentTime + 5);
            break;
        case "KeyF":
            e.preventDefault();
            if (document.fullscreenElement) {
                document.exitFullscreen();
            } else {
                video.requestFullscreen();
            }
            break;
        case "KeyM":
            e.preventDefault();
            video.muted = !video.muted;
            break;
    }
});

video.addEventListener("error", function () {
    const container = document.querySelector(".player-container");
    container.innerHTML = `
        <div class="error-state">
            <div class="error-icon">⚠</div>
            <h2>Couldn't load video</h2>
            <p>Instagram may have blocked this request, or the video may be private.</p>
            <a href="/" class="retry-link">Try another URL</a>
        </div>
    `;
});

// Copy link button
(function () {
    const btn = document.getElementById("copyLinkBtn");
    const toast = document.getElementById("copyToast");
    if (!btn) return;

    let toastTimer = null;

    function showToast(msg) {
        toast.textContent = msg;
        toast.classList.add("show");
        clearTimeout(toastTimer);
        toastTimer = setTimeout(() => toast.classList.remove("show"), 2000);
    }

    btn.addEventListener("click", async function () {
        const url = window.location.href;
        try {
            await navigator.clipboard.writeText(url);
            const icon = btn.querySelector(".copy-icon");
            const orig = icon.textContent;
            icon.textContent = "✓";
            setTimeout(() => { icon.textContent = orig; }, 1500);
            showToast("Link copied!");
        } catch {
            // Fallback for older browsers
            const ta = document.createElement("textarea");
            ta.value = url;
            ta.style.position = "fixed";
            ta.style.opacity = "0";
            document.body.appendChild(ta);
            ta.select();
            try {
                document.execCommand("copy");
                showToast("Link copied!");
            } catch {
                showToast("Copy failed — please copy the URL manually.");
            }
            document.body.removeChild(ta);
        }
    });
}());
