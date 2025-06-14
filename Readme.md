# Docker Container Management System

## Project Overview
The **Docker Container Management System** is developed in Golang, offering both a REST API and a web interface for managing Docker containers, images, networks, and volumes with ease and efficiency.

---

## 🧩 Features

### 🐳 Container Management
- **Create containers**: Automatically create containers from images with custom settings  
- **Start containers**: Launch existing containers  
- **Stop containers**: Pause running containers  
- **Remove containers**: Delete containers from the system  
- **Smart port mapping**: Automatically select available ports if requested ones are in use  
- **View logs**: Display logs of containers  
- **Execute commands**: Run shell commands inside running containers  
- **Bulk operations**: Perform actions (start, stop, remove) on multiple containers at once  

### 📦 Image Management
- **List images**: Show all Docker images on the system  
- **Search images**: Search for Docker images on Docker Hub  
- **Pull images**: Download images from a Docker registry  
- **Delete images**: Remove unwanted or unused images  

### 📊 Monitoring & Observability
- **Real-time stats**: Monitor CPU, memory, and disk usage live  
- **Container-level metrics**: View status and resource consumption for each container  
- **Dashboard**: Visual overview of system health and activity  
- **Alerts**: Notify when containers crash or resources exceed thresholds  
- **Historical data**: Track and visualize usage trends over time  
- **Usage reports**: Generate reports for performance and resource usage  

### ⚙️ System Management
- **System stats**: Display information about CPU, memory, and disk  
- **System cleanup**: Remove unused containers, images to free up space  
- **Network management**: View and manage Docker networks  
- **Volume management**: View and manage Docker volumes  

### 🤖 Smart Handling
- **Port conflict resolution**: Detect and suggest solutions for port conflicts  
- **Detailed error feedback**: Display clear error messages with fix suggestions  
- **Auto-naming**: Automatically generate unique names for containers if none is provided  

---

## 📡 API Endpoints

### 🔧 Container Management
- `POST /create` – Create and start a new container  
- `GET /status` – List all containers  
- `GET /stop/:id` – Stop a container by ID or name  
- `GET /start/:id` – Start a container by ID or name  
- `GET /remove/:id` – Remove a container by ID or name  
- `GET /logs/:id` – View logs of a container  
- `POST /exec/:id` – Execute command inside a container  
- `POST /bulk/:action` – Perform bulk operations (`start`, `stop`, `remove`, `restart`)  

### 📁 Image Management
- `GET /images` – List all Docker images  
- `POST /images/pull` – Pull image from registry  
- `DELETE /images/:id` – Delete image by ID or name  
- `GET /images/search/:term` – Search for image on Docker Hub  

### 🧠 System Management
- `GET /stats` – System statistics (containers, images, CPU, memory, disk)  
- `POST /cleanup` – Clean up unused resources  
- `GET /networks` – List Docker networks  
- `GET /volumes` – List Docker volumes  

---

## ⚙️ Requirements
- Golang 1.16 or higher  
- Docker Engine  
- Access to Docker daemon socket (e.g., `/var/run/docker.sock`)

---

## 🚀 Usage

1. Launch the application and access the web UI at:  
   **http://localhost:8081**

2. Use REST API endpoints (e.g. via Postman or curl) to control Docker containers and resources.

---

## 🤝 Contributing

Contributions are welcome! Please submit a pull request or open an issue to participate in development or suggest improvements.

---

## 📄 License
Copyright (c) 2025 Bùi Minh Thành. All rights reserved.
