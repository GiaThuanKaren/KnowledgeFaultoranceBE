# FeaziestFlow Backend API (`kfdesktopbe`)

Dịch vụ Backend REST API hiệu năng cao viết bằng Golang cho ứng dụng FeaziestFlow (Knowledge Fault Tolerance), tuân thủ triết lý "Feasible, Easiest, Flow".

> 📖 **Dành Cho Lập Trình Viên:**
> Xem chi tiết các bước cài đặt và khởi chạy API trong lúc phát triển tại: [Hướng Dẫn Setup Dev (docs/SETUP_DEV.md)](file:///D:/Workspace/Feaziest/KnowledgeFaultorance/kfdesktopbe/docs/SETUP_DEV.md).

---

## 1. Công Nghệ Sử Dụng (Tech Stack)

- **Ngôn ngữ:** Golang 1.22+
- **HTTP Routing:** `go-chi/chi/v5`
- **Database Driver:** `jackc/pgx/v5` (Pure SQL, tuyệt đối KHÔNG dùng ORM)
- **Database Code Generator:** `sqlc`
- **Cache & Rate Limiting:** `redis/go-redis/v9` (Redis 7)
- **Authentication:** Firebase Admin SDK (`firebase.google.com/go/v4`)
- **Logging:** Structured logging qua thư viện chuẩn `log/slog`
- **Containerization:** Multi-stage Docker, Docker Compose

---

## 2. Cấu Trúc Thư Mục (Project Structure)

```text
kfdesktopbe/
├── cmd/
│   └── server/
│       └── main.go                 # Entrypoint server: graceful shutdown, init db/redis/router
├── internal/
│   ├── config/                     # Load config từ .env và environment variables
│   ├── database/                   # Khởi tạo pgxpool, Redis client và migrations
│   │   ├── migrations/             # Schema migration thuần SQL (000001_init_schema.up.sql)
│   │   └── queries/                # File truy vấn SQL thuần cho sqlc
│   ├── db/                         # Code Go sinh bởi sqlc
│   ├── domain/                     # Entity models và contract interfaces
│   ├── repository/                 # Repository layer bọc sqlc queries
│   ├── service/                    # Business logic (ownership check, soft delete, links parsing)
│   ├── middleware/                 # Firebase Auth, Redis Rate Limiter, Slog, Timeout, CORS
│   └── handler/                    # HTTP Handlers (Chi router) & chuẩn hóa JSON response
├── Dockerfile                      # Multi-stage production container
├── docker-compose.yml              # Local / VPS development stack (Postgres, Redis, App)
├── fly.toml                        # Cấu hình triển khai Fly.io
├── render.yaml                     # Cấu hình triển khai Render Blueprint
├── Makefile                        # Lệnh build, migrate, test, sqlc-gen
├── .env.example                    # File mẫu biến môi trường
├── .gitignore                      # Loại bỏ triệt để file credentials/secret
├── go.mod
└── go.sum
```

---

## 3. Cấu Hình Biến Môi Trường (Environment Variables)

Hệ thống tải cấu hình từ biến môi trường hệ thống hoặc file `.env`.

> **BẢO MẬT QUAN TRỌNG:**
> Tuyệt đối KHÔNG commit file `.env` hoặc file chứa secret/service account key lên Git repository. File `.gitignore` đã được cấu hình tự động loại bỏ các file nhạy cảm này.

1. Sao chép file cấu hình mẫu:
   ```bash
   cp .env.example .env
   ```

2. Các biến môi trường chính:
   | Biến môi trường | Mặc định | Chú thích |
   | :--- | :--- | :--- |
   | `PORT` | `8080` | Port lắng nghe của HTTP Server |
   | `ENVIRONMENT` | `development` | Môi trường (`development` / `production` / `test`) |
   | `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/kfdesktop?sslmode=disable` | Connection string PostgreSQL (Ưu tiên cao nhất) |
   | `DB_HOST` | `localhost` | Host PostgreSQL (khi không dùng `DATABASE_URL`) |
   | `DB_PORT` | `5432` | Port PostgreSQL |
   | `DB_USER` | `postgres` | User PostgreSQL |
   | `DB_PASSWORD` | `postgres` | Password PostgreSQL |
   | `DB_NAME` | `kfdesktop` | Tên cơ sở dữ liệu PostgreSQL |
   | `DB_SSLMODE` | `disable` | Chế độ SSL (`disable` / `require`) |
   | `REDIS_URL` | `redis://localhost:6379/0` | URL kết nối Redis (Ưu tiên cao nhất) |
   | `REDIS_HOST` | `localhost` | Host Redis (khi không dùng `REDIS_URL`) |
   | `REDIS_PORT` | `6379` | Port Redis |
   | `REDIS_PASSWORD` | `""` | Password Redis |
   | `FIREBASE_CREDENTIALS_JSON` | `""` | JSON string Service Account của Firebase |

---

## 4. Chạy Ứng Dụng (Local Development)

### Cách 1: Sử dụng Docker Compose (Khuyên Dùng)
Khởi động đồng thời PostgreSQL 16, Redis 7 và Backend API:
```bash
docker compose up -d
```
Kiểm tra log:
```bash
docker compose logs -f backend-api
```
Dừng hệ thống:
```bash
docker compose down
```

### Cách 2: Chạy Trực Tiếp Native Binary
Yêu cầu đã cài đặt Go 1.22+ và đã có PostgreSQL & Redis đang chạy:
```bash
# 1. Cài đặt dependencies
go mod download

# 2. Chạy server trực tiếp
go run ./cmd/server

# Hoặc dùng Makefile
make run
```

---

## 5. Cơ Sở Dữ Liệu & Migrations (Pure SQL)

Lược đồ cơ sở dữ liệu được định nghĩa bằng SQL thuần trong thư mục `internal/database/migrations/`:
- `000001_init_schema.up.sql`: Tạo các bảng `users`, `projects`, `notes`, `note_links`, `daily_stats` cùng các chỉ mục tối ưu (Partial index `WHERE deleted_at IS NULL`).
- `000001_init_schema.down.sql`: Rollback xoá các bảng.

Khi khởi chạy bằng `docker compose`, thư mục migrations tự động được nạp vào `/docker-entrypoint-initdb.d` để thiết lập cơ sở dữ liệu ban đầu.

---

## 6. Kiểm Thử (Testing)

Chạy bộ unit test toàn diện:
```bash
go test -v ./...
```
Hoặc dùng make:
```bash
make test
```

Chạy bộ kiểm thử End-to-End (E2E) Sanity Check:
```bash
# Standalone in-process test
go run scripts/verify_e2e.go

# Hoặc kiểm thử server đang chạy
go run scripts/verify_e2e.go -url http://localhost:8080
```

---

## 7. Triển Khai Lên Cloud (Cloud Deployments)

### Fly.io
Dự án đã có sẵn file `fly.toml`:
```bash
fly launch
fly deploy
```

### Render
Dự án đã có sẵn file `render.yaml` (Blueprint). Chỉ cần kết nối repo với Render và chọn New Blueprint Instance.

### Standalone Docker Image
Build container image sản xuất siêu nhẹ:
```bash
docker build -t kfdesktopbe:latest .
```
