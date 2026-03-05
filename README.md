# InstaWatch

InstaWatch is a simple, self-hosted web application that allows you to watch Instagram videos directly from your browser. It securely downloads the video using [yt-dlp](https://github.com/yt-dlp/yt-dlp), optimizes the file format for seamless web playback across devices, and serves it through a clean, custom player.

## Running with Docker Compose

Running InstaWatch via Docker Compose is the easiest and recommended approach. 

### Prerequisites

* Docker
* Docker Compose

### Instructions

1. Clone this repository to your local machine:

```bash
git clone <repository_url>
cd instawatch
```

2. (Optional but recommended) If you want to configure an Instagram session cookie to avoid rate limits or download restricted videos, create a `.env` file in the root of the project:

```text
INSTAGRAM_SESSION_ID=your_sessionid_cookie_value
```

You can find your session ID by inspecting the cookies of your browser when you are logged into Instagram.

3. Start the container in the background:

```bash
docker compose up -d
```

4. Open your web browser and navigate to `http://localhost:8080`.

### Disclaimer

This project is not affiliated with Instagram or any other social media platform. It is a simple, self-hosted web application that allows you to watch Instagram videos directly from your browser. It securely downloads the video using yt-dlp, optimizes the file format for seamless web playback across devices, and serves it through a clean, custom player.

This project is not intended to be used for commercial purposes. It is intended to be used for personal purposes only.
