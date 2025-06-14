# Docker Container Management System

## Tổng quan dự án (Project Overview)
Hệ thống quản lý Docker Container được phát triển bằng Golang, cung cấp REST API và giao diện web để quản lý các container, image, network và volume trong Docker một cách đơn giản và hiệu quả.

## Tính năng phần mềm (Software Features)

### Quản lý Container
- **Tạo container**: Tự động tạo container từ image với cấu hình tùy chỉnh
- **Khởi động container**: Bật container đã tạo
- **Dừng container**: Tạm dừng container đang chạy
- **Xóa container**: Loại bỏ container khỏi hệ thống
- **Ánh xạ cổng thông minh**: Tự động chọn cổng trống nếu cổng yêu cầu đã được sử dụng
- **Xem logs**: Hiển thị logs của container
- **Thực thi lệnh**: Chạy lệnh bên trong container đang hoạt động
- **Thao tác hàng loạt**: Thực hiện hành động trên nhiều container cùng lúc

### Quản lý Image
- **Liệt kê images**: Hiển thị tất cả Docker images hiện có trên hệ thống
- **Tìm kiếm images**: Tìm kiếm images từ Docker Hub
- **Tải image**: Tải xuống images từ registry
- **Xóa image**: Loại bỏ image không cần thiết

### Giám sát và Monitoring
- **Thống kê tài nguyên**: Theo dõi CPU, RAM và ổ đĩa theo thời gian thực
- **Giám sát container**: Hiển thị trạng thái và tài nguyên của từng container
- **Dashboard tổng quan**: Bảng điều khiển trực quan hiển thị tình trạng hệ thống
- **Cảnh báo**: Thông báo khi tài nguyên vượt ngưỡng hoặc container gặp sự cố
- **Thống kê lịch sử**: Lưu trữ và hiển thị dữ liệu hiệu suất theo thời gian
- **Báo cáo sử dụng**: Tạo báo cáo về việc sử dụng tài nguyên theo thời gian

### Quản lý hệ thống
- **Thống kê hệ thống**: Hiển thị thông tin về CPU, bộ nhớ, ổ đĩa
- **Dọn dẹp hệ thống**: Xóa container, images không sử dụng để giải phóng không gian
- **Quản lý network**: Xem và quản lý các mạng Docker
- **Quản lý volume**: Xem và quản lý các volume lưu trữ

### Xử lý thông minh
- **Xử lý xung đột port**: Tự động phát hiện và đề xuất giải pháp khi có xung đột port
- **Báo lỗi chi tiết**: Cung cấp thông báo lỗi rõ ràng với hướng dẫn khắc phục
- **Tự động đánh tên**: Tự động tạo tên duy nhất cho container nếu không được cung cấp

## API Endpoints

### Container Management
- `POST /create`: Tạo và khởi động container mới
- `GET /status`: Liệt kê tất cả containers
- `GET /stop/:id`: Dừng container theo ID hoặc tên
- `GET /start/:id`: Khởi động container theo ID hoặc tên
- `GET /remove/:id`: Xóa container theo ID hoặc tên
- `GET /logs/:id`: Xem logs của container
- `POST /exec/:id`: Thực thi lệnh trong container
- `POST /bulk/:action`: Thực hiện hành động hàng loạt (start, stop, remove, restart)

### Image Management
- `GET /images`: Liệt kê tất cả Docker images
- `POST /images/pull`: Tải Docker image từ registry
- `DELETE /images/:id`: Xóa Docker image theo ID hoặc tên
- `GET /images/search/:term`: Tìm kiếm image trên Docker Hub

### System Management
- `GET /stats`: Thống kê hệ thống (container, image, CPU, RAM, ổ đĩa)
- `POST /cleanup`: Dọn dẹp hệ thống
- `GET /networks`: Liệt kê Docker networks
- `GET /volumes`: Liệt kê Docker volumes

## Yêu cầu hệ thống (Requirements)
- Go 1.16 hoặc cao hơn
- Docker Engine
- Quyền truy cập vào Docker daemon socket


## Sử dụng (Usage)
1. Truy cập giao diện web tại http://localhost:8081
2. Sử dụng các API endpoints để quản lý Docker qua REST API

## Đóng góp (Contributing)
Mọi đóng góp đều được hoan nghênh. Vui lòng tạo pull request hoặc issue để đóng góp vào dự án.

## Giấy phép (License)
Copyright (c) 2025 Bùi Minh Thành. All rights reserved.
