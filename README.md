
![Logo](assets/logo.png)
## What is Donetick
An open-source, user-friendly app for managing tasks and chores, featuring customizable options to help you and others stay organized

## Features Highlights
- Task and Chore Management: Easily create, edit, and manage tasks and chores for yourself or your group.
- Build with sharing in mind: you give people access to you group and you can assign each other tasks. only those assigned to task or chore can see it
- Assignee Assignment: Assign tasks to specific individuals with ability rotate them automatically using customizable strategies like randomly, least completed,etc..
- Recurring Tasks: Schedule tasks to repeat daily, weekly, monthly, or yearly or something to trigger on specific day of month or day of the week. if you are not sure you can have adaptive recurring task where it does figure it out base on historical completion 
- Progress Tracking: Track the completion status of tasks and view historical data.
- API Integration 


## Selfhosted : 
Release binary included everything needed to be up and running, as even the frontend file served 
### Using Docker run :
1. pull the latest image using: `docker pull donetick/donetick`
2. run the container `DT_ENV=selfhosted docker run -v /path/to/host/data:/usr/src/app/data -p 2021:2021 donetick/donetick`


### Using Docker Compose:
You can use the following template 
```yaml
services:
  donetick:
    image: donetick/donetick
    container_name: donetick
    restart: unless-stopped
    ports:
      - 2021:2021 # needed for serving backend and frontend
    volumes:
      - ./data:/usr/src/app/data # database file stored (sqlite database)
      - ./config:/config # configration file like selfhosted.yaml
    environment:
      - DT_ENV=selfhosted # this tell donetick to load ./config/selfhosted.yaml for the configuration file

```

### Using binary:


## Development Environment 
1. Clone the repository:
2. Navigate to the project directory: `cd donetick`
3. Download dependency `go mod download`
4. Run locally `go run .`


## Contributing
Contributions are welcome! If you would like to contribute to Donetick, please follow these steps:
1. Fork the repository
2. Create a new branch: `git checkout -b feature/your-feature-name`
3. Make your changes and commit them: `git commit -m 'Add some feature'`
4. Push to the branch: `git push origin feature/your-feature-name`
5. Submit a pull request


## License
This project is licensed under the AGPLv3. See the [LICENSE](LICENSE) file for more details. I might consider changing it later to something else


