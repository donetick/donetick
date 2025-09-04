
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
**Collaborative**: Create and manage tasks either solo or with family and friends. You can create a group and share or assign some of the tasks or chores with others.

**Natural Language Task Creation**: Describe what you need to do in plain English. Donetick automatically extracts dates, times, and recurrence patterns from phrases like “Change water filter every 6 months” or “Take the trash out every Monday and Tuesday at 6:15 pm.”

**Task Advanced Scheduling**: 
- Supports flexible scheduling: daily, weekly, monthly, yearly, specific months, specific days of the week, or even adaptive scheduling — where Donetick learns from historical completions to suggest due dates automatically.
- Due Date vs Completion Date Based Recurrence: Choose whether recurring tasks should be scheduled from the previous due date (ideal for a consistent cadence) or from the actual completion date (useful when tasks are often delayed).
- Assignee Rotation: Automatically rotate task assignments based on who has completed the fewest tasks, randomly, or in turns(round-robin) order.
- Time Tracking & Session Insights: Track how much time you spend on a task whether in a single session or across multiple.
  
**Subtasks with Smart Reset**: Break tasks into smaller steps with subtasks, each trackable on its own. For recurring tasks, subtasks automatically reset when the main task is completed. subtasks can be nested as well!

**Organize with Priorities and Labels**: Organize everything using custom labels and priorities. Labels can be shared across your group, making it easy to filter and sort tasks by category. Priorities help you stay focused  Donetick supports five levels: P1, P2, P3, P4, and No Priority.

**Add Photos**: Attach photos directly to tasks. Supports local storage (WIP) or cloud providers including AWS S3, Cloudflare R2, MinIO, and other S3-compatible services.

**Things**: A unique feature in Donetick. “Things” let you track data that isn’t a task. A Thing can be a number, boolean (true/false), or plain text. You can also mark tasks as done automatically when a Thing changes to a certain value.

**NFC Tag Support**: Create physical triggers by writing NFC tags that instantly mark tasks as complete when scanned.

### Gamification & Progress
**Points System**: Built-in points system that rewards task completion and tracks your progress over time.

**Completion Restrictions** : You can restrict task completion until a certain time, for example, make a task completable only within the last X hours before its due date. This helps prevent marking tasks as "done" too early.

**Comprehensive Analytics**: See task breakdowns by label, completion status, and other helpful graphs.

### Security & Authentication
**Multi-Factor Authentication**: Supports TOTP-based MFA.

**Multiple Sign-In Options**: Choose from local accounts or any OAuth2 provider that supports OIDC, like Keycloak, Authentik, Authelia, etc. (Tested with Authentik.)

### Notifications & Integrations

**Dashboard View**: If you’re on a larger screen (like a laptop or tablet) and logged in as an admin, Donetick shows a mount-friendly dashboard layout. a full task list, calendar, and recent activity all in one place. Perfect for wall-mounted displays or shared tablets. With the ability for any user to pick their account and complete a task on the go!

**Realtime Sync**: Enable realtime sync to instantly reflect task changes across all connected devices and users.  whether you are adding, editing, or completing a task. It reflects immediately on enabled devices!

**Offline Support**: You can access donetick if you lose connection and navigate some areas, but this is very limited functionality at the moment. 

**Multi-Platform Notifications**: Get reminders through the mobile app (we have an alpha iOS app on TestFlight, and the Android APK is available in releases), as well as via Telegram, Discord, or Pushover.

**Home Assistant Integration**: Manage and view tasks directly within Home Assistant using the official integration. It creates separate to-do lists for each Donetick user. Donetick Home Assistant Integration

### Developer & API Features
**REST API**: Full access to Donetick’s features through a REST API, great for custom automations and integrations. (For external use, we recommend using the eAPI, which offers limited access intended for long-lived access tokens.)

**Webhook System**: Connect Donetick to external systems using flexible webhook support good for custom notification flows or automations.

---

## Quick Start
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



## Development Environment

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

## Contributing

Contributions are welcome! If you want to work on something that is not listed as an issue, please open a [Discussion](https://github.com/donetick/donetick/discussions) first to ensure it aligns with our goals and to avoid any unnecessary effort!

---

## License

This project is licensed under the **AGPLv3**. See the [LICENSE](LICENSE) file for more details.

---

## Join the Discussion
For ideas or feature requests, please use GitHub Discussions. We also have a Discord server and a subreddit for those who prefer those platforms!


[![Discord](https://img.shields.io/discord/1272383484509421639)](https://discord.gg/6hSH6F33q7)
[![Reddit](https://img.shields.io/reddit/subreddit-subscribers/donetick)](https://www.reddit.com/r/donetick)

[![Github Discussion](https://img.shields.io/github/discussions/donetick/donetick)](https://github.com/donetick/donetick/discussions)

---

## Support Donetick

 If you find it helpful, consider supporting us by starring the repository, contributing code, or sharing feedback!  

---
