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
