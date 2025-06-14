/*
 * Docker Container Management System
 * Copyright (c) 2025 Bùi Minh Thành
 * All rights reserved.
 *
 * This software is the proprietary information of Bùi Minh Thành.
 * Use is subject to license terms.
 */

package main

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
)

type CreateContainerRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Port  string `json:"port"`
}

type ImageRequest struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

func main() {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	// Add CORS middleware for better API compatibility
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "index.html", gin.H{
			"message": "Docker management system",
		})
	})

	r.POST("/create", func(ctx *gin.Context) {
		var req CreateContainerRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
			return
		}

		// Log the request for debugging
		fmt.Printf("Creating container: name=%s, image=%s, port=%s\n", req.Name, req.Image, req.Port)

		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			fmt.Printf("Error creating Docker client: %v\n", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		// Check if Docker daemon is accessible
		_, err = cli.Ping(context)
		if err != nil {
			fmt.Printf("Error pinging Docker daemon: %v\n", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		imageName := req.Image
		if imageName == "" {
			imageName = "nginx:latest"
		}

		fmt.Printf("Pulling image: %s\n", imageName)

		// Check if image already exists locally first
		images, err := cli.ImageList(context, image.ListOptions{})
		if err != nil {
			fmt.Printf("Error listing images: %v\n", err)
		} else {
			imageExists := false
			for _, img := range images {
				for _, tag := range img.RepoTags {
					if tag == imageName {
						imageExists = true
						fmt.Printf("Image %s already exists locally\n", imageName)
						break
					}
				}
				if imageExists {
					break
				}
			}

			// Only pull if image doesn't exist locally
			if !imageExists {
				fmt.Printf("Image %s not found locally, pulling from registry\n", imageName)
				reader, err := cli.ImagePull(context, imageName, image.PullOptions{})
				if err != nil {
					fmt.Printf("Error pulling image: %v\n", err)
					ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error pulling image: " + err.Error()})
					return
				}
				defer reader.Close()

				// Read the pull output to complete the operation
				_, err = io.Copy(io.Discard, reader)
				if err != nil {
					fmt.Printf("Error reading pull output: %v\n", err)
					ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading pull output: " + err.Error()})
					return
				}
				fmt.Printf("Successfully pulled image: %s\n", imageName)
			}
		}

		// Generate unique container name
		containerName := req.Name
		if containerName == "" {
			containerName = "my-container-" + strconv.FormatInt(time.Now().Unix(), 10)
		} else {
			// Check if container name already exists
			containers, err := cli.ContainerList(context, container.ListOptions{All: true})
			if err == nil {
				for _, c := range containers {
					for _, name := range c.Names {
						if strings.TrimPrefix(name, "/") == containerName {
							// Add timestamp to make it unique
							containerName = containerName + "-" + strconv.FormatInt(time.Now().Unix(), 10)
							fmt.Printf("Container name conflict, using: %s\n", containerName)
							break
						}
					}
				}
			}
		}

		// Configure container
		containerConfig := &container.Config{
			Image: imageName,
			Tty:   true,
		}

		// Configure host (port mapping)
		hostConfig := &container.HostConfig{}
		actualPortMapping := "none"
		if req.Port != "" {
			portParts := strings.Split(req.Port, ":")
			if len(portParts) == 2 {
				requestedHostPort := portParts[0]
				containerPort := portParts[1]

				fmt.Printf("Requested port mapping: %s:%s\n", requestedHostPort, containerPort)

				hostPortInt, err := strconv.Atoi(requestedHostPort)
				if err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid host port: " + requestedHostPort})
					return
				}

				// Check if host port is already in use
				isPortInUse := func(port int) bool {
					// Check if it's the server port
					if port == 8080 {
						return true
					}

					// Check existing containers
					containers, err := cli.ContainerList(context, container.ListOptions{All: true})
					if err != nil {
						return false
					}

					for _, c := range containers {
						for _, p := range c.Ports {
							if p.PublicPort != 0 && int(p.PublicPort) == port {
								return true
							}
						}
					}
					return false
				}

				finalHostPort := requestedHostPort

				// Find available port if current one is in use
				if isPortInUse(hostPortInt) {
					fmt.Printf("⚠️  Port %d is already in use, searching for alternative port...\n", hostPortInt)
					foundPort := false

					// Try ports from requested port + 1 to 9999
					for i := hostPortInt + 1; i <= 9999; i++ {
						if !isPortInUse(i) {
							finalHostPort = strconv.Itoa(i)
							foundPort = true
							fmt.Printf("✅ Found available port: %s (original %s was in use)\n", finalHostPort, requestedHostPort)
							break
						}
					}

					// If not found in the above range, try 8081-9999
					if !foundPort {
						for i := 8081; i <= 9999; i++ {
							if !isPortInUse(i) {
								finalHostPort = strconv.Itoa(i)
								foundPort = true
								fmt.Printf("✅ Found available port: %s (fallback range)\n", finalHostPort)
								break
							}
						}
					}

					if !foundPort {
						errorMsg := fmt.Sprintf("Port %s đã được sử dụng và không tìm thấy port thay thế khả dụng", requestedHostPort)
						suggestion := "Hãy thử: sudo netstat -tulpn | grep :" + requestedHostPort + " để xem service nào đang dùng port này"

						fmt.Printf("❌ %s\n", errorMsg)
						ctx.JSON(http.StatusConflict, gin.H{
							"error":          errorMsg,
							"details":        fmt.Sprintf("Đã kiểm tra range %d-9999 và 8081-9999 nhưng không có port nào khả dụng", hostPortInt+1),
							"suggestion":     suggestion,
							"requested_port": requestedHostPort,
							"conflict_type":  "port_unavailable",
							"next_steps": []string{
								"Dừng service đang sử dụng port " + requestedHostPort,
								"Hoặc chọn port khác (ví dụ: 9001:80)",
								"Hoặc để trống để hệ thống tự động chọn port",
							},
						})
						return
					}
				}

				containerConfig.ExposedPorts = nat.PortSet{
					nat.Port(containerPort + "/tcp"): struct{}{},
				}

				hostConfig.PortBindings = nat.PortMap{
					nat.Port(containerPort + "/tcp"): []nat.PortBinding{
						{
							HostIP:   "0.0.0.0",
							HostPort: finalHostPort,
						},
					},
				}

				actualPortMapping = finalHostPort + ":" + containerPort
				fmt.Printf("✅ Final port mapping configured: %s\n", actualPortMapping)
			}
		}

		fmt.Printf("Creating container with name: %s\n", containerName)

		resp, err := cli.ContainerCreate(context, containerConfig, hostConfig, nil, nil, containerName)
		if err != nil {
			fmt.Printf("❌ Error creating container: %v\n", err)

			// If still conflict, try with timestamp
			if strings.Contains(err.Error(), "already in use") {
				if strings.Contains(err.Error(), "container name") {
					containerName = containerName + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
					fmt.Printf("🔄 Retrying with unique name: %s\n", containerName)
					resp, err = cli.ContainerCreate(context, containerConfig, hostConfig, nil, nil, containerName)
				} else if strings.Contains(err.Error(), "bind host port") {
					// Extract port from error message
					portFromError := "unknown"
					if strings.Contains(err.Error(), ":") {
						parts := strings.Split(err.Error(), ":")
						for _, part := range parts {
							if len(part) > 0 && part[0] >= '0' && part[0] <= '9' {
								portFromError = strings.Fields(part)[0]
								break
							}
						}
					}

					ctx.JSON(http.StatusConflict, gin.H{
						"error":         fmt.Sprintf("Không thể tạo container: Port %s đã được sử dụng bởi service khác", portFromError),
						"details":       "Đây có thể là service hệ thống (không phải Docker container)",
						"suggestion":    "sudo lsof -i :" + portFromError + " hoặc sudo netstat -tulpn | grep :" + portFromError,
						"conflict_type": "system_port_conflict",
						"port_in_use":   portFromError,
						"solution_options": []string{
							"Dừng service đang sử dụng port " + portFromError,
							"Sử dụng port khác cho container",
							"Sử dụng port mapping khác (ví dụ: 9001:" + strings.Split(actualPortMapping, ":")[1] + ")",
						},
					})
					return
				}
			}

			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating container: " + err.Error()})
				return
			}
		}

		fmt.Printf("✅ Container created with ID: %s, starting...\n", resp.ID)

		if err := cli.ContainerStart(context, resp.ID, container.StartOptions{}); err != nil {
			fmt.Printf("❌ Error starting container: %v\n", err)

			// Parse error for more specific information
			errorDetails := err.Error()
			var conflictPort string
			var conflictType string

			if strings.Contains(errorDetails, "bind host port") {
				conflictType = "port_binding_failed"
				// Extract port from error
				if strings.Contains(errorDetails, "0.0.0.0:") {
					start := strings.Index(errorDetails, "0.0.0.0:") + 8
					end := strings.Index(errorDetails[start:], ":")
					if end > 0 {
						conflictPort = errorDetails[start : start+end]
					}
				}
			} else if strings.Contains(errorDetails, "address already in use") {
				conflictType = "address_in_use"
			}

			if conflictType != "" {
				ctx.JSON(http.StatusConflict, gin.H{
					"error":            "Không thể khởi động container do xung đột port",
					"details":          fmt.Sprintf("Port %s đang được sử dụng bởi service khác trên hệ thống", conflictPort),
					"suggestion":       "sudo lsof -i :" + conflictPort + " để xem service nào đang dùng port",
					"container_id":     resp.ID,
					"conflict_type":    conflictType,
					"port_in_conflict": conflictPort,
					"note":             "Container đã được tạo nhưng không thể khởi động. Bạn có thể xóa nó trong danh sách container.",
					"recommended_actions": []string{
						"Kiểm tra service đang sử dụng port: sudo lsof -i :" + conflictPort,
						"Dừng service đó nếu không cần thiết",
						"Hoặc xóa container này và tạo lại với port khác",
						"Hoặc sử dụng docker port mapping khác",
					},
				})
				return
			}

			// Generic error for other cases
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error":        "Lỗi khởi động container",
				"details":      errorDetails,
				"container_id": resp.ID,
				"suggestion":   "Kiểm tra logs container để biết thêm chi tiết",
			})
			return
		}

		fmt.Printf("🎉 Container %s started successfully on port %s\n", containerName, actualPortMapping)

		// Return detailed response
		response := gin.H{
			"message": "Container created and started successfully! 🎉",
			"id":      resp.ID,
			"name":    containerName,
			"image":   imageName,
			"port":    actualPortMapping,
		}

		if actualPortMapping != req.Port && req.Port != "" {
			response["note"] = fmt.Sprintf("⚠️ Port was automatically changed from %s to %s due to conflict", req.Port, actualPortMapping)
			response["original_port"] = req.Port
		}

		ctx.JSON(http.StatusOK, response)
	})

	r.GET("/status", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		// Check if Docker daemon is accessible
		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible. Please start Docker service: " + err.Error()})
			return
		}

		// Get ALL containers (running and stopped) by setting All: true
		containers, err := cli.ContainerList(context, container.ListOptions{All: true})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing containers: " + err.Error()})
			return
		}

		if len(containers) == 0 {
			ctx.JSON(http.StatusOK, gin.H{"message": "No containers found", "containers": []interface{}{}})
			return
		}

		ctx.JSON(http.StatusOK, containers)
	})

	r.GET("/stop/:id", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		// Check if Docker daemon is accessible
		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		containerID := ctx.Param("id")

		// Try to find container by name or ID
		containers, err := cli.ContainerList(context, container.ListOptions{All: true})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing containers: " + err.Error()})
			return
		}

		var targetContainer string
		for _, c := range containers {
			if c.ID == containerID || c.ID[:12] == containerID {
				targetContainer = c.ID
				break
			}
			for _, name := range c.Names {
				if strings.TrimPrefix(name, "/") == containerID {
					targetContainer = c.ID
					break
				}
			}
		}

		if targetContainer == "" {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Container not found: " + containerID})
			return
		}

		if err := cli.ContainerStop(context, targetContainer, container.StopOptions{}); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error stopping container: " + err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"message": "Container " + containerID + " stopped successfully"})
	})

	r.GET("/start/:id", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		// Check if Docker daemon is accessible
		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		containerID := ctx.Param("id")
		fmt.Printf("Starting container: %s\n", containerID)

		// Try to find container by name or ID
		containers, err := cli.ContainerList(context, container.ListOptions{All: true})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing containers: " + err.Error()})
			return
		}

		var targetContainer string
		var targetContainerName string
		for _, c := range containers {
			if c.ID == containerID || c.ID[:12] == containerID {
				targetContainer = c.ID
				if len(c.Names) > 0 {
					targetContainerName = strings.TrimPrefix(c.Names[0], "/")
				}
				break
			}
			for _, name := range c.Names {
				if strings.TrimPrefix(name, "/") == containerID {
					targetContainer = c.ID
					targetContainerName = strings.TrimPrefix(name, "/")
					break
				}
			}
		}

		if targetContainer == "" {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error":      "Container not found: " + containerID,
				"suggestion": "Vui lòng kiểm tra lại Container ID hoặc tên container",
			})
			return
		}

		// Check current container status
		containerInfo, err := cli.ContainerInspect(context, targetContainer)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error inspecting container: " + err.Error()})
			return
		}

		if containerInfo.State.Running {
			ctx.JSON(http.StatusConflict, gin.H{
				"error":          fmt.Sprintf("Container '%s' is already running", targetContainerName),
				"details":        "Container đã đang chạy, không cần khởi động lại",
				"current_status": "running",
			})
			return
		}

		// Start the container
		if err := cli.ContainerStart(context, targetContainer, container.StartOptions{}); err != nil {
			fmt.Printf("Error starting container: %v\n", err)

			// Handle specific errors
			errorDetails := err.Error()
			if strings.Contains(errorDetails, "bind host port") || strings.Contains(errorDetails, "address already in use") {
				// Extract port from error
				var conflictPort string
				if strings.Contains(errorDetails, "0.0.0.0:") {
					start := strings.Index(errorDetails, "0.0.0.0:") + 8
					end := strings.Index(errorDetails[start:], ":")
					if end > 0 {
						conflictPort = errorDetails[start : start+end]
					}
				}

				ctx.JSON(http.StatusConflict, gin.H{
					"error":            "Không thể khởi động container do xung đột port",
					"details":          fmt.Sprintf("Port %s đang được sử dụng bởi service khác", conflictPort),
					"suggestion":       "sudo lsof -i :" + conflictPort + " để kiểm tra service nào đang sử dụng port",
					"conflict_type":    "port_conflict",
					"port_in_conflict": conflictPort,
					"recommended_actions": []string{
						"Dừng service đang sử dụng port " + conflictPort,
						"Hoặc sử dụng port mapping khác cho container",
						"Hoặc dừng container khác đang sử dụng port này",
					},
				})
				return
			}

			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error":          "Error starting container: " + err.Error(),
				"container_name": targetContainerName,
				"suggestion":     "Kiểm tra logs container để xem chi tiết lỗi",
			})
			return
		}

		fmt.Printf("✅ Container %s started successfully\n", targetContainerName)
		ctx.JSON(http.StatusOK, gin.H{
			"message":        fmt.Sprintf("🚀 Container '%s' started successfully!", targetContainerName),
			"container_id":   targetContainer[:12],
			"container_name": targetContainerName,
		})
	})

	r.GET("/remove/:id", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		// Check if Docker daemon is accessible
		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		containerID := ctx.Param("id")

		// Try to find container by name or ID
		containers, err := cli.ContainerList(context, container.ListOptions{All: true})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing containers: " + err.Error()})
			return
		}

		var targetContainer string
		for _, c := range containers {
			if c.ID == containerID || c.ID[:12] == containerID {
				targetContainer = c.ID
				break
			}
			for _, name := range c.Names {
				if strings.TrimPrefix(name, "/") == containerID {
					targetContainer = c.ID
					break
				}
			}
		}

		if targetContainer == "" {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Container not found: " + containerID})
			return
		}

		if err := cli.ContainerRemove(context, targetContainer, container.RemoveOptions{Force: true}); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error removing container: " + err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"message": "Container " + containerID + " removed successfully"})
	})

	// Add image management endpoints
	r.GET("/images", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		images, err := cli.ImageList(context, image.ListOptions{})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing images: " + err.Error()})
			return
		}

		if len(images) == 0 {
			ctx.JSON(http.StatusOK, gin.H{"message": "No images found", "images": []interface{}{}})
			return
		}

		ctx.JSON(http.StatusOK, images)
	})

	r.POST("/images/pull", func(ctx *gin.Context) {
		var req ImageRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
			return
		}

		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		imageName := req.Name
		if req.Tag != "" {
			imageName = req.Name + ":" + req.Tag
		}

		if imageName == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Image name is required"})
			return
		}

		reader, err := cli.ImagePull(context, imageName, image.PullOptions{})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error pulling image: " + err.Error()})
			return
		}
		defer reader.Close()

		// Read the pull output (optional - for logging)
		_, err = io.Copy(io.Discard, reader)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading pull output: " + err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"message": "Image pulled successfully",
			"image":   imageName,
		})
	})

	r.DELETE("/images/:id", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		imageID := ctx.Param("id")

		// Try to remove the image directly first (handles full image names like nginx:latest)
		_, err = cli.ImageRemove(context, imageID, image.RemoveOptions{Force: true})
		if err == nil {
			ctx.JSON(http.StatusOK, gin.H{"message": "Image " + imageID + " removed successfully"})
			return
		}

		// If direct removal fails, try to find image by ID or tag
		images, err := cli.ImageList(context, image.ListOptions{})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing images: " + err.Error()})
			return
		}

		var targetImage string
		for _, img := range images {
			// Check full ID match
			if img.ID == imageID || img.ID == "sha256:"+imageID {
				targetImage = img.ID
				break
			}
			// Check truncated ID match
			if strings.HasPrefix(img.ID, "sha256:"+imageID) || strings.HasPrefix(img.ID, imageID) {
				targetImage = img.ID
				break
			}
			// Check RepoTags
			for _, tag := range img.RepoTags {
				if tag == imageID {
					targetImage = img.ID
					break
				}
				// Also check if imageID matches repository part
				if strings.Contains(tag, imageID) {
					targetImage = img.ID
					break
				}
			}
			if targetImage != "" {
				break
			}
		}

		if targetImage == "" {
			// List available images for debugging
			var availableImages []string
			for _, img := range images {
				for _, tag := range img.RepoTags {
					availableImages = append(availableImages, tag)
				}
				availableImages = append(availableImages, img.ID[:12])
			}
			ctx.JSON(http.StatusNotFound, gin.H{
				"error":            "Image not found: " + imageID,
				"available_images": availableImages,
				"suggestion":       "Try using the exact image name from the list or the image ID",
			})
			return
		}

		_, err = cli.ImageRemove(context, targetImage, image.RemoveOptions{Force: true})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error removing image: " + err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "Image " + imageID + " removed successfully"})
	})

	// Add image search endpoint
	r.GET("/images/search/:term", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		searchTerm := ctx.Param("term")
		if searchTerm == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Search term is required"})
			return
		}

		// Search for images on Docker Hub
		searchResults, err := cli.ImageSearch(context, searchTerm, registry.SearchOptions{Limit: 25})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error searching images: " + err.Error()})
			return
		}

		if len(searchResults) == 0 {
			ctx.JSON(http.StatusOK, gin.H{"message": "No images found", "results": []interface{}{}})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"results": searchResults})
	})

	// Add system statistics endpoint with system info
	r.GET("/stats", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon. Is Docker running? " + err.Error()})
			return
		}
		defer cli.Close()

		_, err = cli.Ping(context)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Docker daemon is not accessible: " + err.Error()})
			return
		}

		// Get containers
		containers, err := cli.ContainerList(context, container.ListOptions{All: true})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing containers: " + err.Error()})
			return
		}

		// Get images
		images, err := cli.ImageList(context, image.ListOptions{})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing images: " + err.Error()})
			return
		}

		// Get system info
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// Get disk usage
		var diskStats syscall.Statfs_t
		syscall.Statfs("/", &diskStats)
		diskTotal := diskStats.Blocks * uint64(diskStats.Bsize)
		diskFree := diskStats.Bavail * uint64(diskStats.Bsize)
		diskUsed := diskTotal - diskFree

		// Get CPU count
		cpuCount := runtime.NumCPU()

		// Calculate statistics
		stats := gin.H{
			"containers": gin.H{
				"total":   len(containers),
				"running": 0,
				"stopped": 0,
				"paused":  0,
			},
			"images": gin.H{
				"total": len(images),
			},
			"system": gin.H{
				"timestamp": time.Now(),
				"memory": gin.H{
					"total":   memStats.Sys,
					"used":    memStats.Alloc,
					"free":    memStats.Sys - memStats.Alloc,
					"percent": float64(memStats.Alloc) / float64(memStats.Sys) * 100,
				},
				"disk": gin.H{
					"total":   diskTotal,
					"used":    diskUsed,
					"free":    diskFree,
					"percent": float64(diskUsed) / float64(diskTotal) * 100,
				},
				"cpu": gin.H{
					"cores": cpuCount,
				},
			},
		}

		// Count container states
		for _, c := range containers {
			switch c.State {
			case "running":
				stats["containers"].(gin.H)["running"] = stats["containers"].(gin.H)["running"].(int) + 1
			case "exited", "created":
				stats["containers"].(gin.H)["stopped"] = stats["containers"].(gin.H)["stopped"].(int) + 1
			case "paused":
				stats["containers"].(gin.H)["paused"] = stats["containers"].(gin.H)["paused"].(int) + 1
			}
		}

		ctx.JSON(http.StatusOK, stats)
	})

	// Add container logs endpoint
	r.GET("/logs/:id", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon: " + err.Error()})
			return
		}
		defer cli.Close()

		containerID := ctx.Param("id")
		tailLines := ctx.DefaultQuery("tail", "100")

		logs, err := cli.ContainerLogs(context, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       tailLines,
			Timestamps: true,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting logs: " + err.Error()})
			return
		}
		defer logs.Close()

		logContent, err := io.ReadAll(logs)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading logs: " + err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"logs":      string(logContent),
			"container": containerID,
		})
	})

	// Add container exec endpoint
	r.POST("/exec/:id", func(ctx *gin.Context) {
		var req struct {
			Command string `json:"command"`
		}
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
			return
		}

		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon: " + err.Error()})
			return
		}
		defer cli.Close()

		containerID := ctx.Param("id")

		execConfig := container.ExecOptions{
			Cmd:          []string{"sh", "-c", req.Command},
			AttachStdout: true,
			AttachStderr: true,
		}

		execResp, err := cli.ContainerExecCreate(context, containerID, execConfig)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating exec: " + err.Error()})
			return
		}

		resp, err := cli.ContainerExecAttach(context, execResp.ID, container.ExecStartOptions{})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error starting exec: " + err.Error()})
			return
		}
		defer resp.Close()

		output, err := io.ReadAll(resp.Reader)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading output: " + err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"output":    string(output),
			"command":   req.Command,
			"container": containerID,
		})
	})

	// Add bulk operations endpoint
	r.POST("/bulk/:action", func(ctx *gin.Context) {
		var req struct {
			Containers []string `json:"containers"`
		}
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
			return
		}

		action := ctx.Param("action")
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon: " + err.Error()})
			return
		}
		defer cli.Close()

		results := make(map[string]interface{})
		successCount := 0
		errorCount := 0

		for _, containerID := range req.Containers {
			var err error

			switch action {
			case "start":
				err = cli.ContainerStart(context, containerID, container.StartOptions{})
			case "stop":
				timeout := 30 // 30 seconds timeout
				err = cli.ContainerStop(context, containerID, container.StopOptions{Timeout: &timeout})
			case "remove":
				err = cli.ContainerRemove(context, containerID, container.RemoveOptions{Force: true})
			case "restart":
				timeout := 30 // 30 seconds timeout
				err = cli.ContainerRestart(context, containerID, container.StopOptions{Timeout: &timeout})
			default:
				err = fmt.Errorf("unknown action: %s", action)
			}

			if err != nil {
				results[containerID] = gin.H{"status": "error", "message": err.Error()}
				errorCount++
				fmt.Printf("❌ Bulk %s failed for container %s: %v\n", action, containerID, err)
			} else {
				results[containerID] = gin.H{"status": "success"}
				successCount++
				fmt.Printf("✅ Bulk %s succeeded for container %s\n", action, containerID)
			}
		}

		fmt.Printf("📦 Bulk %s completed: %d success, %d errors\n", action, successCount, errorCount)

		ctx.JSON(http.StatusOK, gin.H{
			"action":  action,
			"results": results,
			"summary": gin.H{
				"total":   len(req.Containers),
				"success": successCount,
				"errors":  errorCount,
			},
		})
	})

	// Add system cleanup endpoint
	r.POST("/cleanup", func(ctx *gin.Context) {
		cmd := exec.Command("docker", "system", "prune", "-f")
		output, err := cmd.CombinedOutput()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error running cleanup: " + err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"message": "System cleanup completed",
			"output":  string(output),
		})
	})

	// Add network management endpoint
	r.GET("/networks", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon: " + err.Error()})
			return
		}
		defer cli.Close()

		networks, err := cli.NetworkList(context, network.ListOptions{})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing networks: " + err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, networks)
	})

	// Add volume management endpoint
	r.GET("/volumes", func(ctx *gin.Context) {
		context := ctx.Request.Context()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot connect to Docker daemon: " + err.Error()})
			return
		}
		defer cli.Close()

		volumes, err := cli.VolumeList(context, volume.ListOptions{})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error listing volumes: " + err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, volumes)
	})

	// Serve static files
	r.Static("/static", "./static")
	// Serve HTML templates
	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	// Listen and serve on port 8080
	r.Run(":8081")
}
