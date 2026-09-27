# Hướng Dẫn Sử Dụng & Tạo Tài Liệu Swagger (`/api/docs`)

Tài liệu này hướng dẫn cách truy cập giao diện Swagger UI, cách thử nghiệm trực tiếp các API và quy chuẩn viết chú thích (annotations) để tự động sinh Swagger Documentation cho toàn bộ routes trong Backend Golang (`kfdesktopbe`).

---

## 1. Truy Cập Giao Diện Swagger UI

Sau khi khởi động server Backend Go (ví dụ trên cổng `5005` hoặc `8080`), bạn có thể mở Swagger UI trực tiếp trên trình duyệt:

- **Đường dẫn truy cập:**
  ```text
  http://localhost:5005/api/docs
  ```
  *(Hệ thống sẽ tự động chuyển hướng tới `http://localhost:5005/api/docs/index.html`)*

- **Đường dẫn schema JSON thô:**
  ```text
  http://localhost:5005/api/docs/doc.json
  ```

---

## 2. Cách Tạo & Cập Nhật Swagger Doc Tự Động (Generate Docs)

Mỗi khi bạn **thêm mới** hoặc **sửa đổi** bất kỳ route / handler / model DTO nào, chỉ cần chạy một trong 2 câu lệnh sau để tự động cập nhật lại toàn bộ tài liệu Swagger:

### Cách 1: Dùng lệnh Make (Nhanh nhất)
```bash
make swagger
```

### Cách 2: Dùng lệnh `go run` trực tiếp (Không cần cài CLI toàn cục)
```bash
go run github.com/swaggo/swag/cmd/swag init -g cmd/server/main.go -o docs
```

> [!TIP]
> Lệnh trên sẽ tự động duyệt toàn bộ các hàm handler và models trong dự án để tái tạo 3 file bên trong thư mục `docs/`:
> - `docs/docs.go`: Mã Go nhúng dữ liệu Swagger vào binary
> - `docs/swagger.json`: Đặc tả định dạng OpenAPI/Swagger JSON
> - `docs/swagger.yaml`: Đặc tả định dạng YAML

---

## 3. Cú Pháp Viết Chú Thích (Swagger Annotations Template)

Để Swagger tự động nhận diện và hiển thị một route, bạn chỉ cần đặt khối chú thích ngay phía trên hàm Handler theo chuẩn sau:

### Mẫu 1: API Yêu Cầu Body JSON & Có Xác Thực (Ví dụ POST)
```go
// CreateProject handles POST /api/v1/projects
// @Summary      Tạo mới một dự án workspace
// @Description  Tạo dự án mới thuộc quyền sở hữu của người dùng hiện tại
// @Tags         projects
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      domain.CreateProjectRequest  true  "Dữ liệu tạo dự án"
// @Success      201      {object}  domain.Project
// @Failure      400      {object}  ErrorEnvelope
// @Failure      401      {object}  ErrorEnvelope
// @Router       /api/v1/projects [post]
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

### Mẫu 2: API Có Path Param & Query Param (Ví dụ GET)
```go
// ListProjects handles GET /api/v1/projects?page=1&limit=20
// @Summary      Duyệt danh sách dự án
// @Description  Lấy danh sách dự án phân trang (loại trừ các dự án đã xóa mềm)
// @Tags         projects
// @Produce      json
// @Security     BearerAuth
// @Param        page   query     int  false  "Số trang (mặc định 1)"
// @Param        limit  query     int  false  "Số lượng mỗi trang (mặc định 20)"
// @Success      200    {object}  ProjectListResponse
// @Failure      401    {object}  ErrorEnvelope
// @Router       /api/v1/projects [get]
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

### Mẫu 3: API Có Path Param (Ví dụ PUT / DELETE)
```go
// GetNote handles GET /api/v1/notes/{noteId}
// @Summary      Chi tiết ghi chú
// @Description  Lấy nội dung ghi chú kèm đồ thị liên kết tri thức 2 chiều
// @Tags         notes
// @Produce      json
// @Security     BearerAuth
// @Param        noteId  path      string  true  "UUID của ghi chú"
// @Success      200     {object}  domain.NoteDetail
// @Failure      401     {object}  ErrorEnvelope
// @Failure      404     {object}  ErrorEnvelope
// @Router       /api/v1/notes/{noteId} [get]
func (h *NoteHandler) GetNote(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

---

## 4. Giải Thích Các Từ Khóa Annotations Quan Trọng

| Từ khóa | Ý nghĩa | Ví dụ |
| :--- | :--- | :--- |
| `@Summary` | Tiêu đề ngắn gọn của endpoint | `@Summary Tạo ghi chú nhanh` |
| `@Description` | Mô tả chi tiết hành vi và logic nghiệp vụ | `@Description Tạo note không gán dự án, độ trễ < 200ms` |
| `@Tags` | Nhóm các API cùng nghiệp vụ trên giao diện UI | `@Tags notes`, `@Tags projects`, `@Tags users` |
| `@Accept` | Định dạng dữ liệu client gửi lên | `json` |
| `@Produce` | Định dạng dữ liệu server trả về | `json` |
| `@Security` | Cơ chế xác thực | `BearerAuth` (yêu cầu Authorization Header) |
| `@Param` | Tham số truyền vào: `<tên> <vị trí> <kiểu dữ liệu> <bắt buộc> <mô tả>` | `@Param noteId path string true "ID của Note"` |
| `@Success` | Mã trạng thái thành công và kiểu dữ liệu trả về | `@Success 200 {object} domain.Note` |
| `@Failure` | Mã trạng thái lỗi và cấu trúc lỗi | `@Failure 401 {object} ErrorEnvelope` |
| `@Router` | Đường dẫn và phương thức HTTP | `@Router /api/v1/notes/quick [post]` |

---

## 5. Hướng Dẫn Thử Nghiệm API Có Xác Thực Trên Swagger UI

Các route thuộc `/api/v1/*` đều được bảo vệ bởi middleware xác thực. Để test trực tiếp trên trình duyệt:

1. Mở trang `http://localhost:5005/api/docs`.
2. Bấm vào nút **Authorize 🔓** (ở góc phải trên cùng của giao diện Swagger).
3. Tại ô `Value`, nhập cú pháp:
   ```text
   Bearer mock-token:<USER_ID>
   ```
   *(Ví dụ: `Bearer mock-token:dev-tester-01`)*
4. Bấm **Authorize** -> Bấm **Close**.
5. Bây giờ bạn có thể mở bất kỳ API nào (ví dụ: `POST /api/v1/notes/quick`), bấm **Try it out**, chỉnh sửa body và bấm **Execute** để xem kết quả trả về `201 Created` ngay lập tức!
