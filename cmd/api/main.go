package main

import (
	"fmt"
	"log"
	"net/http"
	"weaviate-admin/internal/config"
	"weaviate-admin/internal/core"

	_ "weaviate-admin/docs" // QUAN TRỌNG: Import thư mục docs của swag

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// ==========================================
// 1. ĐỊNH NGHĨA STRUCT CHO SWAGGER DOCS
// ==========================================

type ErrorResponse struct {
	Error string `json:"error"`
}

type PingResponse struct {
	Status  string `json:"status" example:"PONG"`
	Message string `json:"message" example:"Weaviate đang hoạt động khỏe mạnh"`
	Error   string `json:"error,omitempty"`
}

type CreateCollectionRequest struct {
	Name string `json:"name" example:"Article"`
}

type AddTenantRequest struct {
	TenantName string `json:"tenant_name" example:"tenant_01"`
}

type CreateBackupRequest struct {
	BackupID string `json:"backup_id" example:"backup-2026-04-17"`
	Backend  string `json:"backend" example:"filesystem"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type RestoreBackupRequest struct {
	Backend string `json:"backend" example:"filesystem"`
}

// ==========================================
// 2. KHỞI TẠO HANDLER
// ==========================================

type APIHandler struct {
	repo *core.WeaviateRepo
}

// ==========================================
// 3. HÀM MAIN ROUTER
// ==========================================

// @title Weaviate Admin API
// @version 1.0
// @description API quản trị hệ thống Weaviate Vector Database.
// @host localhost:8081
// @BasePath /weaviate-admin
func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Không thể load config: %v", err)
	}

	repo, err := core.NewWeaviateRepo(
		cfg.Weaviate.Host,
		cfg.Weaviate.Scheme,
		cfg.Weaviate.APIKey,
	)
	if err != nil {
		log.Fatalf("Lỗi kết nối Weaviate: %v", err)
	}

	handler := &APIHandler{repo: repo}
	e := echo.New()

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:    true,
		LogStatus: true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Printf("[%s] %s | Status: %d\n", v.Method, v.URI, v.Status)
			return nil
		},
	}))
	e.Use(middleware.Recover())

	// Khởi tạo Group với tiền tố /weaviate-admin
	apiGroup := e.Group("/weaviate-admin")

	// Route cho Swagger UI
	apiGroup.GET("/swagger/*", echoSwagger.WrapHandler)

	// Các API cũ
	apiGroup.GET("/ping", handler.Ping)
	apiGroup.GET("/cluster/nodes", handler.GetNodes)
	apiGroup.POST("/collections", handler.CreateCollection)
	apiGroup.POST("/collections/:name/tenants", handler.AddTenant)

	// Các API mới bổ sung
	apiGroup.GET("/collections/:name", handler.GetCollection)
	apiGroup.GET("/collections/:name/tenants", handler.GetTenants)
	apiGroup.GET("/collections/:name/objects", handler.GetObjects)
	apiGroup.POST("/backups", handler.CreateBackup)
	apiGroup.POST("/backups/:id/restore", handler.RestoreBackup)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", cfg.Server.Port)))
}

// ==========================================
// 4. LOGIC CÁC API HANDLERS
// ==========================================

// Ping godoc
// @Summary Kiểm tra trạng thái Weaviate
// @Description Gọi ping và liveness/readiness probe
// @Tags System
// @Produce json
// @Success 200 {object} PingResponse
// @Failure 503 {object} PingResponse
// @Router /ping [get]
func (h *APIHandler) Ping(c echo.Context) error {
	ctx := c.Request().Context()

	// 1. Kiểm tra Liveness
	isLive, liveErr := h.repo.Ping(ctx)
	if liveErr != nil || !isLive {
		errMsg := ""
		if liveErr != nil {
			errMsg = liveErr.Error()
		}
		return c.JSON(http.StatusServiceUnavailable, PingResponse{
			Status:  "DOWN",
			Message: "Weaviate không phản hồi",
			Error:   errMsg,
		})
	}

	// 2. Kiểm tra Readiness
	isReady, readyErr := h.repo.IsReady(ctx)
	if readyErr != nil || !isReady {
		return c.JSON(http.StatusServiceUnavailable, PingResponse{
			Status:  "STARTING",
			Message: "Weaviate đang chạy nhưng chưa sẵn sàng (đang nạp Index)",
		})
	}

	return c.JSON(http.StatusOK, PingResponse{
		Status:  "PONG",
		Message: "Weaviate đang hoạt động khỏe mạnh",
	})
}

// GetNodes godoc
// @Summary Lấy danh sách Nodes
// @Description Trả về trạng thái của các node trong Weaviate cluster
// @Tags Cluster
// @Produce json
// @Success 200 {array} object
// @Failure 500 {object} ErrorResponse
// @Router /cluster/nodes [get]
func (h *APIHandler) GetNodes(c echo.Context) error {
	nodes, err := h.repo.GetNodes(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, nodes)
}

// CreateCollection godoc
// @Summary Tạo Collection
// @Description Khởi tạo một Collection mới
// @Tags Collections
// @Accept json
// @Produce json
// @Param request body CreateCollectionRequest true "Tên Collection"
// @Success 201 {object} MessageResponse
// @Failure 400,500 {object} ErrorResponse
// @Router /collections [post]
func (h *APIHandler) CreateCollection(c echo.Context) error {
	var req CreateCollectionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
	}

	err := h.repo.CreateCollection(c.Request().Context(), req.Name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusCreated, MessageResponse{Message: "Created collection " + req.Name})
}

// AddTenant godoc
// @Summary Thêm Tenant
// @Description Đăng ký một tenant mới vào collection
// @Tags Collections
// @Accept json
// @Produce json
// @Param name path string true "Tên Collection"
// @Param request body AddTenantRequest true "Thông tin Tenant"
// @Success 200 {object} MessageResponse
// @Failure 400,500 {object} ErrorResponse
// @Router /collections/{name}/tenants [post]
func (h *APIHandler) AddTenant(c echo.Context) error {
	className := c.Param("name")
	var req AddTenantRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
	}

	err := h.repo.AddTenant(c.Request().Context(), className, req.TenantName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, MessageResponse{Message: "Tenant " + req.TenantName + " added successfully"})
}

// GetCollection godoc
// @Summary Lấy thông tin Collection
// @Description Xem chi tiết schema (vectorizer, properties) của một Collection
// @Tags Collections
// @Produce json
// @Param name path string true "Tên Collection"
// @Success 200 {object} object
// @Failure 500 {object} ErrorResponse
// @Router /collections/{name} [get]
func (h *APIHandler) GetCollection(c echo.Context) error {
	className := c.Param("name")
	classObj, err := h.repo.GetCollection(c.Request().Context(), className)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, classObj)
}

// GetTenants godoc
// @Summary Xem danh sách Tenants
// @Description Lấy danh sách toàn bộ tenants thuộc về một Collection
// @Tags Collections
// @Produce json
// @Param name path string true "Tên Collection"
// @Success 200 {array} object
// @Failure 500 {object} ErrorResponse
// @Router /collections/{name}/tenants [get]
func (h *APIHandler) GetTenants(c echo.Context) error {
	className := c.Param("name")
	tenants, err := h.repo.GetTenants(c.Request().Context(), className)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, tenants)
}

// GetObjects godoc
// @Summary Xem dữ liệu Objects
// @Description Trình duyệt các bản ghi có trong Collection
// @Tags Data
// @Produce json
// @Param name path string true "Tên Collection"
// @Param limit query int false "Giới hạn số lượng kết quả (Mặc định: 10)" default(10)
// @Success 200 {array} object
// @Failure 500 {object} ErrorResponse
// @Router /collections/{name}/objects [get]
func (h *APIHandler) GetObjects(c echo.Context) error {
	className := c.Param("name")

	var limit int
	if err := echo.QueryParamsBinder(c).Int("limit", &limit).BindError(); err != nil {
		limit = 10
	}

	objects, err := h.repo.GetObjects(c.Request().Context(), className, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, objects)
}

// CreateBackup godoc
// @Summary Tạo bản Backup
// @Description Yêu cầu Weaviate đóng gói snapshot toàn bộ dữ liệu ra backend cấu hình sẵn
// @Tags System
// @Accept json
// @Produce json
// @Param request body CreateBackupRequest true "Thông tin Backup"
// @Success 202 {object} MessageResponse
// @Failure 400,500 {object} ErrorResponse
// @Router /backups [post]
func (h *APIHandler) CreateBackup(c echo.Context) error {
	var req CreateBackupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
	}

	err := h.repo.CreateBackup(c.Request().Context(), req.Backend, req.BackupID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusAccepted, MessageResponse{Message: "Tiến trình backup đã được khởi chạy"})
}

// RestoreBackup godoc
// @Summary Phục hồi bản Backup
// @Description Yêu cầu Weaviate phục hồi dữ liệu từ một bản backup (snapshot) đã tồn tại
// @Tags System
// @Accept json
// @Produce json
// @Param id path string true "ID của bản Backup cần phục hồi"
// @Param request body RestoreBackupRequest true "Thông tin Backend lưu trữ"
// @Success 202 {object} MessageResponse
// @Failure 400,500 {object} ErrorResponse
// @Router /backups/{id}/restore [post]
func (h *APIHandler) RestoreBackup(c echo.Context) error {
	backupID := c.Param("id") // Lấy backupID từ URL path

	var req RestoreBackupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
	}

	err := h.repo.RestoreBackup(c.Request().Context(), req.Backend, backupID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	// Trả về 202 Accepted vì quá trình load dữ liệu từ ổ cứng vào RAM có thể mất thời gian
	return c.JSON(http.StatusAccepted, MessageResponse{Message: "Tiến trình phục hồi dữ liệu bản " + backupID + " đã được khởi chạy"})
}
