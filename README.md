
# <img src="assets/icon.png" alt="drawing" width="45"/>Donetick 



**Simplify Tasks & Chores, Together!**

Donetick is an open-source, user-friendly app designed to help you organize tasks and chores effectively. featuring customizable options to help you and others stay organized

![Screenshot](assets/screenshot.png)


![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/donetick/donetick/go-release.yml)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/donetick/donetick)
![Docker Pulls](https://img.shields.io/docker/pulls/donetick/donetick)


[![Discord](https://img.shields.io/discord/1272383484509421639)](https://discord.gg/6hSH6F33q7)
[![Reddit](https://img.shields.io/reddit/subreddit-subscribers/donetick)](https://www.reddit.com/r/donetick)

---

## Features

### Task & Chore Management
**Collaborative Organization**: Create and manage tasks either solo or with family and friends through shared circles. Each circle functions as a collaborative workspace with role-based permissions.

**Natural Language Task Creation**: Simply describe what you need to do in plain English. Donetick automatically extracts dates, times, and recurrence patterns from phrases like "Change water filter every 6 months" or "Take the trash out every Monday and Tuesday at 6:15 pm."

**Advanced Scheduling**: Beyond basic recurring tasks, Donetick offers adaptive scheduling that learns from completion patterns, flexible recurrence options, completion windows, and custom triggers based on historical data.

**Subtasks & Organization**: Break down complex tasks into manageable subtasks with progress tracking. Organize everything with labels that can be shared across your circle for consistent categorization.

### Gamification & Progress
**Points System**: a built-in points system that rewards task completion and tracks your productivity over time.

**Comprehensive Analytics**: Monitor completion rates, view historical trends, and analyze productivity patterns to optimize your workflow.

### Security & Authentication
**Multi-Factor Authentication**: Support for TOTP-based MFA.

**Multiple Sign-In Options**: Choose from local accounts, Google OAuth, or other OAuth2 providers for convenient and secure authentication.

### File Management & Storage
**File Attachments**: Attach files directly to tasks and chores. Upload profile photos and manage all your content with ease.

**Flexible Storage**: Works with local storage(WIP) or cloud providers including AWS S3, Cloudflare R2, MinIO, and other S3-compatible services.

### Notifications & Integrations
**Multi-Platform Notifications**: Receive reminders through Telegram, Discord, or Pushover, ensuring you never miss important tasks.

**NFC Tag Support**: Create physical triggers by writing NFC tags that instantly mark tasks as complete when scanned.

**Home Assistant Integration**: View and manage tasks directly within Home Assistant using the official custom component.

**Webhook System**: Connect with external systems through comprehensive webhook support and event-driven architecture.

### Developer & API Features
**REST API**: Full programmatic access to all features through a REST API, perfect for custom integrations and automation.

**Things Integration**: Connect IoT devices and external systems using entities (numbers, strings, booleans) that can trigger tasks, track sensor values, or integrate with smart home systems.

**External Triggers**: Allow other applications and services to create, update, and complete tasks through API endpoints.

🔑 SSO/OIDC Support: Integrate with identity providers using Single Sign-On and OpenID Connect.


---

## 🚀 Quick Start
> [!NOTE]
> Before running the application, ensure you have a valid `selfhosted.yaml` configuration file. 
> If you don't have one, create a `selfhosted.yaml` file based on the example provided [here](https://github.com/donetick/donetick/blob/main/config/selfhosted.yaml).
> Place the `selfhosted.yaml` file in the `/config` directory within your application's root directory 



### Using Docker
1. **Pull the latest image:**
   ```bash
   docker pull donetick/donetick
   ```
2. **Run the container:** Replace `/path/to/host/data` with your preferred data directory:
   ```bash
   docker run -v /path/to/host/data:/donetick-data -p 2021:2021 \
     -e DT_ENV=selfhosted \
     -e DT_SQLITE_PATH=/donetick-data/donetick.db \
     donetick/donetick
   ```

### Using Docker Compose
Use this template to set up Donetick with Docker Compose:
```yaml
services:
  donetick:
    image: donetick/donetick
    container_name: donetick
    restart: unless-stopped
    ports:
      - 2021:2021
    volumes:
      - ./data:/donetick-data
      - ./config:/config
    environment:
      - DT_ENV=selfhosted
      - DT_SQLITE_PATH=/donetick-data/donetick.db
      
```


### Using the Binary
1. **Download the latest release** from the [Releases](https://github.com/donetick/donetick/releases) page.
2. **Extract the file** and navigate to the folder:
   ```bash
   cd path/to/extracted-folder
   ```
3. **Run Donetick:**
   ```bash
   DT_ENV=selfhosted ./donetick 
   ```

---



## 🛠️ Development Environment

### Build the frontend

1. Clone the frontend repository:
   ```bash
   git clone https://github.com/donetick/frontend.git donetick-frontend
   ```
2. Navigate to the frontend directory:
   ```bash
   cd donetick-frontend
   ```
3. Install dependencies:
   ```bash
   npm install
   ```
4. Build the frontend:
   ```bash
   npm run build-selfhosted
   ```

### Build the application

1. Clone the repository:
   ```bash
   git clone https://github.com/donetick/donetick.git
   ```
2. Navigate to the project directory:
   ```bash
   cd donetick
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Copy the frontend build to the application:
   ```bash
   rm -rf ./frontend/dist
   cp -r ../donetick-frontend/dist ./frontend
   ```
5. Run the app locally:
   ```bash
   go run .
   ```
   Or build the application:
   ```bash
   go build -o donetick .
   ```

### Build the development Docker image

> Make sure to build the frontend and the app first before building the Docker image.

1. Build the Docker image:
   ```bash
   docker build -t donetick/donetick -f Dockerfile.dev .
   ```

---

## 🤝 Contributing

Contributions are welcome! If you want to work on something that is not listed as an issue, please open a [Discussion](https://github.com/donetick/donetick/discussions) first to ensure it aligns with our goals and to avoid any unnecessary effort!

if you have an idea also feel free to use the [Discussion](https://github.com/donetick/donetick/discussions)
1. Pick an issue or open discuss about the contribution
2. Fork the repository.
3. Create a new branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```
4. Make your changes and commit them:
   ```bash
   git commit -m 'Add a new feature'
   ```
5. Push your branch:
   ```bash
   git push origin feature/your-feature-name
   ```
6. Submit a pull request.

---

## 🔒 License

This project is licensed under the **AGPLv3**. See the [LICENSE](LICENSE) file for more details.

---

## 💬 Join the Discussion
For ideas or feature requests, please use GitHub Discussions. We also have a Discord server and a subreddit for those who prefer those platforms!


[![Discord](https://img.shields.io/discord/1272383484509421639)](https://discord.gg/6hSH6F33q7)
[![Reddit](https://img.shields.io/reddit/subreddit-subscribers/donetick)](https://www.reddit.com/r/donetick)

[![Github Discussion](https://img.shields.io/github/discussions/donetick/donetick)](https://github.com/donetick/donetick/discussions)

---

## 💡 Support Donetick

 If you find it helpful, consider supporting us by starring the repository, contributing code, or sharing feedback!  

---
