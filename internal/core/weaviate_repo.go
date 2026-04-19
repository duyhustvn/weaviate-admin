package core

import (
	"context"
	"fmt"
	"time"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"
)

type WeaviateRepo struct {
	client *weaviate.Client
}

// Ping kiểm tra xem server Weaviate có phản hồi không (Liveness)
func (r *WeaviateRepo) Ping(ctx context.Context) (bool, error) {
	live, err := r.client.Misc().LiveChecker().Do(ctx)
	if err != nil {
		return false, err
	}
	return live, nil
}

// IsReady kiểm tra xem Weaviate đã load xong dữ liệu và sẵn sàng nhận query chưa
func (r *WeaviateRepo) IsReady(ctx context.Context) (bool, error) {
	ready, err := r.client.Misc().ReadyChecker().Do(ctx)
	if err != nil {
		return false, err
	}
	return ready, nil
}

func NewWeaviateRepo(host, scheme, apiKey string) (*WeaviateRepo, error) {
	cfg := weaviate.Config{
		Host:   host,
		Scheme: scheme,
	}
	if apiKey != "" {
		cfg.Headers = map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", apiKey),
		}
	} else {
		return nil, fmt.Errorf("API key is required")
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("không thể khởi tạo weaviate client: %w", err)
	}

	fmt.Println("Khởi tạo weaviate client thành công")
	// Thiết lập timeout 5 giây cho việc kiểm tra kết nối ban đầu
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Sử dụng LiveChecker để xác nhận server đang sống
	isLive, err := client.Misc().LiveChecker().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("weaviate không phản hồi (ping failed): %w", err)
	}
	fmt.Println("Weaviate ping success")

	if !isLive {
		return nil, fmt.Errorf("weaviate client đã kết nối nhưng server báo trạng thái không sẵn sàng")
	}
	fmt.Println("Weaviate is live and ready")

	return &WeaviateRepo{client: client}, nil
}

// --- CLUSTER MANAGEMENT ---
func (r *WeaviateRepo) GetNodes(ctx context.Context) ([]*models.NodeStatus, error) {
	nodes, err := r.client.Cluster().NodesStatusGetter().Do(ctx)
	if err != nil {
		return nil, err
	}
	return nodes.Nodes, nil
}

// --- COLLECTION MANAGEMENT ---
func (r *WeaviateRepo) CreateCollection(ctx context.Context, className string) error {
	classObj := &models.Class{
		Class:      className,
		Vectorizer: "text2vec-openai", // Mặc định dùng OpenAI
		Properties: []*models.Property{
			{Name: "content", DataType: []string{"text"}},
			{Name: "metadata", DataType: []string{"string"}},
		},
		MultiTenancyConfig: &models.MultiTenancyConfig{Enabled: true}, // Bật Multi-tenancy
	}
	return r.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
}

func (r *WeaviateRepo) ListCollections(ctx context.Context) ([]*models.Class, error) {
	schema, err := r.client.Schema().Getter().Do(ctx)
	if err != nil {
		return nil, err
	}
	return schema.Classes, nil
}

func (r *WeaviateRepo) GetCollection(ctx context.Context, className string) (*models.Class, error) {
	return r.client.Schema().ClassGetter().WithClassName(className).Do(ctx)
}

// --- MULTI-TENANCY ---
func (r *WeaviateRepo) AddTenant(ctx context.Context, className string, tenantName string) error {
	tenants := []models.Tenant{{Name: tenantName}}
	return r.client.Schema().TenantsCreator().
		WithClassName(className).
		WithTenants(tenants...).
		Do(ctx)
}

func (r *WeaviateRepo) GetTenants(ctx context.Context, className string) ([]models.Tenant, error) {
	return r.client.Schema().TenantsGetter().WithClassName(className).Do(ctx)
}

// --- DATA BROWSER & CRUD ---
func (r *WeaviateRepo) GetObjects(ctx context.Context, className string, limit int) ([]*models.Object, error) {
	if limit <= 0 {
		limit = 10
	}
	return r.client.Data().ObjectsGetter().
		WithClassName(className).
		WithLimit(limit).
		Do(ctx)
}

func (r *WeaviateRepo) UpdateMetadata(ctx context.Context, className, id string, properties map[string]interface{}) error {
	return r.client.Data().Updater().
		WithClassName(className).
		WithID(id).
		WithProperties(properties).
		Do(ctx)
}

// --- IMPORT / EXPORT (BACKUP) ---
func (r *WeaviateRepo) CreateBackup(ctx context.Context, backend string, backupID string) error {
	_, err := r.client.Backup().Creator().
		WithBackend(backend).
		WithBackupID(backupID).
		Do(ctx)
	return err
}

// RestoreBackup phục hồi dữ liệu từ bản backup đã có
func (r *WeaviateRepo) RestoreBackup(ctx context.Context, backend string, backupID string) error {
	_, err := r.client.Backup().Restorer().
		WithBackend(backend).
		WithBackupID(backupID).
		Do(ctx)
	return err
}
