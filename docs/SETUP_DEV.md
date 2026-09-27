# Hướng Dẫn Setup & Chạy Backend API (Golang) Trong Lúc Dev

Tài liệu này hướng dẫn chi tiết các bước thiết lập và khởi chạy Backend API (`kfdesktopbe`) trên môi trường phát triển cục bộ (Local Development).

---

## Mục Lục
1. [Yêu Cầu Hệ Thống](#1-yêu-cầu-hệ-thống)
2. [Cách 1: Khởi Chạy Toàn Bộ Qua Docker Compose (Khuyên Dùng)](#2-cách-1-khởi-chạy-toàn-bộ-qua-docker-compose-khuyên-dùng)
3. [Cách 2: Chạy Trực Tiếp Go Trên Máy Host (Live Coding & Debug)](#3-cách-2-chạy-trực-tiếp-go-trên-máy-host-live-coding--debug)
4. [Kiểm Tra Trạng Thái API (Sanity Check)](#4-kiểm-tra-trạng-thái-api-sanity-check)
5. [Cơ Chế Xác Thực Môi Trường Dev (Firebase Dev Token)](#5-cơ-chế-xác-thực-môi-trường-dev-firebase-dev-token)
6. [Kết Nối Với Ứng Dụng Desktop (`kfappdesktop`)](#6-kết-nối-với-ứng-dụng-desktop-kfappdesktop)

---

## 1. Yêu Cầu Hệ Thống

Tùy theo cách chạy mà bạn cần cài đặt:
- **Bắt buộc:** Docker & Docker Compose (Docker Desktop trên Windows / macOS / Linux).
- **Nếu chạy Go trực tiếp trên máy host:**
  - Go 1.22+ (khuyến nghị 1.24 hoặc 1.25)
  - Git

---

## 2. Cách 1: Khởi Chạy Toàn Bộ Qua Docker Compose (Khuyên Dùng)

Cách này nhanh nhất, tự động dựng sẵn **PostgreSQL 16**, **Redis 7** và **Go API Server** trong mạng nội bộ Docker. Bạn không cần cài PostgreSQL hay Go lên máy host.

### Bước 1: Mở terminal tại thư mục backend
```powershell
cd D:\Workspace\Feaziest\KnowledgeFaultorance\kfdesktopbe
```

### Bước 2: Tạo file `.env` từ file mẫu
```powershell
# Trên PowerShell (Windows)
Copy-Item .env.example .env

# Hoặc Bash / Linux / macOS
cp .env.example .env
```
> [!NOTE]
> File `.env.example` đã cấu hình sẵn các thông số mặc định cho mạng Docker (`postgres:5432` và `redis:6379`).

### Bước 3: Build và khởi động toàn bộ services
```powershell
docker compose up --build
```
*(Nếu muốn chạy nền không chiếm cửa sổ terminal, thêm cờ `-d`: `docker compose up -d --build`)*

### Bước 4: Tắt hệ thống khi dừng làm việc
```powershell
docker compose down
```

---

## 3. Cách 2: Chạy Trực Tiếp Go Trên Máy Host (Live Coding & Debug)

Dùng cách này khi bạn cần chỉnh sửa code Go liên tục, chạy debugger trong VS Code / GoLand, hoặc chạy unit test nhanh.

### Bước 1: Chỉ bật PostgreSQL & Redis bằng Docker
Chạy 2 dịch vụ phụ trợ ở chế độ nền (không chạy container Go để tránh xung đột cổng `8080`):
```powershell
cd D:\Workspace\Feaziest\KnowledgeFaultorance\kfdesktopbe
docker compose up -d postgres redis
```

### Bước 2: Tạo file `.env` trỏ về `localhost`
Tạo file `.env` tại thư mục gốc `kfdesktopbe/` với nội dung sau:
```env
PORT=8080
ENVIRONMENT=development

# Database Postgres (Localhost)
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=kfdesktop
DB_SSLMODE=disable

# Chuỗi kết nối đầy đủ (dự phòng)
DATABASE_URL=postgres://postgres:postgres@localhost:5432/kfdesktop?sslmode=disable

# Redis (Localhost)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_URL=redis://localhost:6379

# Firebase Admin SDK (Để trống để tự động bật DevTokenVerifier cho local)
FIREBASE_CREDENTIALS_JSON=
```

### Bước 3: Chạy database migrations (Khởi tạo bảng)
Chỉ cần chạy một lần khi khởi tạo database mới:
```powershell
# Chạy migration schema qua file SQL thuần
Get-Content internal/database/migrations/000001_init_schema.up.sql | docker exec -i kfdesktopbe-postgres psql -U postgres -d kfdesktop
```

### Bước 4: Chạy server Go
```powershell
go run ./cmd/server
```

Khi server khởi động thành công, bạn sẽ thấy log JSON dạng:
```json
{"time":"2026-09-27T...","level":"INFO","msg":"starting HTTP server","addr":":8080"}
```

---

## 4. Kiểm Tra Trạng Thái API (Sanity Check)

### Kiểm tra probes nhanh bằng trình duyệt hoặc Curl:
- **Liveness probe:** `curl http://localhost:8080/health`  
  👉 Phản hồi: `{"status":"ok"}` (HTTP 200)
- **Readiness probe:** `curl http://localhost:8080/ready`  
  👉 Phản hồi: `{"database":"connected","redis":"connected","status":"ready"}` (HTTP 200)

### Chạy script kiểm thử E2E tự động toàn bộ 8 thao tác chính:
```powershell
go run scripts/verify_e2e.go -url http://localhost:8080
```
Script sẽ tự động kiểm tra:
1. `GET /health` (200 OK)
2. `GET /ready` (Định dạng probe DB & Redis)
3. `POST /api/v1/users/sync` (Đồng bộ người dùng)
4. `POST /api/v1/notes/quick` (Tạo ghi chú nhanh, đo độ trễ < 200ms)
5. `POST /api/v1/projects` & `GET /api/v1/projects` (Tạo và duyệt danh sách dự án)
6. `POST /api/v1/projects/{id}/notes` & `GET /api/v1/notes/{id}` (Tạo note liên kết mạng tri thức)
7. `GET /api/v1/users/contributions?year=2026` (Dữ liệu Heatmap 365 ngày)
8. `DELETE /api/v1/projects/{id}` (Kiểm tra xóa mềm, truy vấn lại ra 404)

---

## 5. Cơ Chế Xác Thực Môi Trường Dev (Firebase Dev Token)

Trong môi trường development (khi biến `FIREBASE_CREDENTIALS_JSON` để trống):
- Hệ thống tự động kích hoạt **`DevTokenVerifier`**.
- Bạn có thể gửi request có header xác thực bằng token giả lập theo cú pháp:
  ```http
  Authorization: Bearer mock-token:<USER_ID>
  ```
  *Ví dụ:* `Authorization: Bearer mock-token:user-dev-123`  
  Server sẽ tự động trích xuất `UID = "user-dev-123"` và cho phép thao tác các API bảo vệ `/api/v1/*` mà không cần tài khoản Firebase thật.

---

## 6. Kết Nối Với Ứng Dụng Desktop (`kfappdesktop`)

1. Khởi động ứng dụng Desktop:
   ```powershell
   cd D:\Workspace\Feaziest\KnowledgeFaultorance\kfappdesktop
   npm run dev
   ```
2. Trong ứng dụng Desktop:
   - Vào mục **Cài đặt** (Settings) -> **Dữ liệu & Bộ nhớ** (Vault & Storage).
   - Kiểm tra mục **Backend API Endpoint**: Mặc định là `http://localhost:8080/api/v1`.
   - Bấm nút **"Kiểm tra kết nối"** (Test connection) -> Ứng dụng sẽ ping kiểm tra và hiển thị thông báo kết nối thành công màu xanh.
