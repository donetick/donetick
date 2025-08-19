# <img src="assets/icon.png" alt="drawing" width="45"/>Donetick

**一起简化任务和家务！**

Donetick 是一款开源、用户友好的应用程序，旨在帮助您有效组织任务和家务，提供可定制的选项，帮助您和他人保持井井有条。

![截图](assets/screenshot.png)

![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/donetick/donetick/go-release.yml)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/donetick/donetick)
![Docker Pulls](https://img.shields.io/docker/pulls/donetick/donetick)

[![Discord](https://img.shields.io/discord/1272383484509421639)](https://discord.gg/6hSH6F33q7)
[![Reddit](https://img.shields.io/reddit/subreddit-subscribers/donetick)](https://www.reddit.com/r/donetick)

---

## 功能

### 任务和家务管理
**协作**：可以单独或与家人朋友一起创建和管理任务。您可以创建一个小组，与他人共享或分配任务或家务。

**自然语言创建任务**：用简单的英语描述您需要做什么。Donetick 会自动从“每6个月更换一次滤水器”或“每周一和周二下午6:15倒垃圾”等短语中提取日期、时间和重复模式。

**高级任务调度**：
- 支持灵活调度：每日、每周、每月、每年、特定月份、特定星期几，甚至自适应调度——Donetick 会从历史完成情况中学习，自动建议截止日期。
- 基于截止日期与完成日期的重复：选择重复任务是应从上一个截止日期开始安排（适合保持一致的节奏），还是从实际完成日期开始安排（适用于任务经常延迟的情况）。
- 执行人轮换：根据完成任务最少、随机或轮流（round-robin）顺序自动轮换任务分配。
- 时间跟踪和会话洞察：跟踪您在单个或多个会话中花费在任务上的时间。

**带智能重置的子任务**：将任务分解为更小的步骤，每个步骤都可以单独跟踪。对于重复性任务，主任务完成后，子任务会自动重置。子任务也可以嵌套！

**使用优先级和标签进行组织**：使用自定义标签和优先级来组织一切。标签可以在您的小组中共享，便于按类别筛选和排序任务。优先级帮助您保持专注。Donetick 支持五个级别：P1、P2、P3、P4和无优先级。

**添加照片**：直接将照片附加到任务中。支持本地存储（开发中）或云提供商，包括 AWS S3、Cloudflare R2、MinIO 和其他S3兼容服务。

**“物品”（Things）**：Donetick 的一个独特功能。“物品”让您跟踪非任务数据。一个“物品”可以是数字、布尔值（真/假）或纯文本。当“物品”变为某个特定值时，您还可以自动将任务标记为完成。

**NFC标签支持**：通过写入NFC标签创建物理触发器，扫描后即可立即将任务标记为完成。

### 游戏化和进度
**积分系统**：内置积分系统，奖励任务完成并跟踪您的长期进度。

**完成限制**：您可以限制任务在某个时间之前不能完成，例如，使任务仅在其截止日期前的最后X小时内可完成。这有助于防止过早将任务标记为“完成”。

**综合分析**：按标签、完成状态和其他有用的图表查看任务分解。

### 安全和认证
**多因素认证**：支持基于TOTP的MFA。

**多种登录选项**：可选择本地帐户或任何支持OIDC的OAuth2提供商，如Keycloak、Authentik、Authelia等。（已通过Authentik测试。）

### 通知和集成

**仪表板视图**：如果您在较大屏幕（如笔记本电脑或平板电脑）上以管理员身份登录，Donetick会显示一个适合挂载的仪表板布局，将完整的任务列表、日历和最近活动集中在一处。非常适合壁挂式显示器或共享平板电脑。任何用户都可以选择自己的帐户并随时完成任务！

**实时同步**：启用实时同步，可在所有连接的设备和用户之间即时反映任务更改，无论是添加、编辑还是完成任务，都会在启用的设备上立即反映出来！

**离线支持**：如果您失去连接，可以访问donetick并浏览某些区域，但目前此功能非常有限。

**多平台通知**：通过移动应用程序（我们在TestFlight上有一个alpha版的iOS应用，Android APK在发布版本中提供）、Telegram、Discord或Pushover获取提醒。

**Home Assistant集成**：使用官方集成直接在Home Assistant中管理和查看任务。它为每个Donetick用户创建单独的待办事项列表。Donetick Home Assistant集成

### 开发者和API功能
**REST API**：通过REST API完全访问Donetick的功能，非常适合自定义自动化和集成。（对于外部使用，我们建议使用eAPI，它提供有限的访问权限，适用于长期访问令牌。）

**Webhook系统**：使用灵活的webhook支持将Donetick连接到外部系统，适用于自定义通知流程或自动化。

---

## 快速入门
> [!NOTE]
> 在运行应用程序之前，请确保您有一个有效的 `selfhosted.yaml` 配置文件。
> 如果您没有，请根据[此处](https://github.com/donetick/donetick/blob/main/config/selfhosted.yaml)提供的示例创建一个 `selfhosted.yaml` 文件。
> 将 `selfhosted.yaml` 文件放置在应用程序根目录下的 `/config` 目录中。

### 使用 Docker
1. **拉取最新镜像：**
   ```bash
   docker pull donetick/donetick
   ```
2. **运行容器：** 将 `/path/to/host/data` 替换为您首选的数据目录：
   ```bash
   docker run -v /path/to/host/data:/donetick-data -p 2021:2021 \
     -e DT_ENV=selfhosted \
     -e DT_SQLITE_PATH=/donetick-data/donetick.db \
     donetick/donetick
   ```

### 使用 Docker Compose
使用此模板通过Docker Compose设置Donetick：
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

### 使用二进制文件
1. **从[发布](https://github.com/donetick/donetick/releases)页面下载最新版本。**
2. **解压文件**并导航到该文件夹：
   ```bash
   cd path/to/extracted-folder
   ```
3. **运行Donetick：**
   ```bash
   DT_ENV=selfhosted ./donetick
   ```

---

## 开发环境

### 构建前端

1. 克隆前端仓库：
   ```bash
   git clone https://github.com/donetick/frontend.git donetick-frontend
   ```
2. 导航到前端目录：
   ```bash
   cd donetick-frontend
   ```
3. 安装依赖：
   ```bash
   npm install
   ```
4. 构建前端：
   ```bash
   npm run build-selfhosted
   ```

### 构建应用程序

1. 克隆仓库：
   ```bash
   git clone https://github.com/donetick/donetick.git
   ```
2. 导航到项目目录：
   ```bash
   cd donetick
   ```
3. 安装依赖：
   ```bash
   go mod download
   ```
4. 将前端构建复制到应用程序：
   ```bash
   rm -rf ./frontend/dist
   cp -r ../donetick-frontend/dist ./frontend
   ```
5. 本地运行应用：
   ```bash
   go run .
   ```
   或构建应用程序：
   ```bash
   go build -o donetick .
   ```

### 构建开发Docker镜像

> 在构建Docker镜像之前，请确保先构建前端和应用程序。

1. 构建Docker镜像：
   ```bash
   docker build -t donetick/donetick -f Dockerfile.dev .
   ```

---

## 贡献

欢迎贡献！如果您想处理未列为问题的内容，请先开启一个[讨论](https://github.com/donetick/donetick/discussions)，以确保它符合我们的目标并避免不必要的努力！

---

## 许可证

本项目采用**AGPLv3**许可证。更多详情请参阅[LICENSE](LICENSE)文件。

---

## 加入讨论
如有想法或功能请求，请使用GitHub讨论。我们还有一个Discord服务器和一个subreddit，供喜欢这些平台的用户使用！

[![Discord](https://img.shields.io/discord/1272383484509421639)](https://discord.gg/6hSH6F33q7)
[![Reddit](https://img.shields.io/reddit/subreddit-subscribers/donetick)](https://www.reddit.com/r/donetick)

[![Github Discussion](https://img.shields.io/github/discussions/donetick/donetick)](https://github.com/donetick/donetick/discussions)

---

## 支持 Donetick

如果您觉得它有帮助，请考虑通过给仓库加星、贡献代码或分享反馈来支持我们！

---
