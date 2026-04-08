package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// KnowledgeBaseRepository 知识库数据访问
type KnowledgeBaseRepository struct {
	db *sql.DB
}

// NewKnowledgeBaseRepository 创建 KnowledgeBase Repository
func NewKnowledgeBaseRepository(db *sql.DB) *KnowledgeBaseRepository {
	return &KnowledgeBaseRepository{db: db}
}

// Create 创建知识库
func (r *KnowledgeBaseRepository) Create(ctx context.Context, kb *model.KnowledgeBase) error {
	query := `
		INSERT INTO knowledge_bases (id, name, display_name, description, type, config, query_endpoint, status, query_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	config, _ := json.Marshal(kb.Config)

	_, err := r.db.ExecContext(ctx, query,
		kb.ID.String(), kb.Name, kb.DisplayName, kb.Description, kb.Type, config, kb.QueryEndpoint, kb.Status, kb.QueryCount, kb.CreatedAt, kb.UpdatedAt,
	)
	return err
}

// scanKnowledgeBase 辅助函数，扫描 KnowledgeBase 行
func scanKnowledgeBase(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.KnowledgeBase, error) {
	kb := &model.KnowledgeBase{}
	var idStr string
	var displayName, description, queryEndpoint sql.NullString
	var config []byte
	var lastQueryAt sql.NullTime

	err := scanner.Scan(
		&idStr, &kb.Name, &displayName, &description, &kb.Type, &config, &queryEndpoint, &kb.Status, &lastQueryAt, &kb.QueryCount, &kb.CreatedAt, &kb.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	kb.ID, _ = uuid.Parse(idStr)
	if displayName.Valid {
		kb.DisplayName = displayName.String
	}
	if description.Valid {
		kb.Description = description.String
	}
	if queryEndpoint.Valid {
		kb.QueryEndpoint = queryEndpoint.String
	}
	if lastQueryAt.Valid {
		kb.LastQueryAt = &lastQueryAt.Time
	}
	json.Unmarshal(config, &kb.Config)

	return kb, nil
}

// FindByID 根据 ID 查找
func (r *KnowledgeBaseRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.KnowledgeBase, error) {
	query := `
		SELECT id, name, display_name, description, type, config, query_endpoint, status, last_query_at, query_count, created_at, updated_at
		FROM knowledge_bases WHERE id = ?
	`
	kb, err := scanKnowledgeBase(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("knowledge base not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find knowledge base: %w", err)
	}
	return kb, nil
}

// FindByName 根据名称查找
func (r *KnowledgeBaseRepository) FindByName(ctx context.Context, name string) (*model.KnowledgeBase, error) {
	query := `
		SELECT id, name, display_name, description, type, config, query_endpoint, status, last_query_at, query_count, created_at, updated_at
		FROM knowledge_bases WHERE name = ?
	`
	kb, err := scanKnowledgeBase(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("knowledge base not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find knowledge base: %w", err)
	}
	return kb, nil
}

// List 列出知识库
func (r *KnowledgeBaseRepository) List(ctx context.Context, query *model.KnowledgeBaseListQuery) ([]*model.KnowledgeBase, int64, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if query.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, query.Type)
	}
	if query.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, query.Status)
	}
	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR display_name LIKE ? OR description LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM knowledge_bases " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count knowledge bases: %w", err)
	}

	// 分页
	page := query.Page
	pageSize := query.Size
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 查询列表
	listQuery := `
		SELECT id, name, display_name, description, type, config, query_endpoint, status, last_query_at, query_count, created_at, updated_at
		FROM knowledge_bases ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list knowledge bases: %w", err)
	}
	defer rows.Close()

	kbs := make([]*model.KnowledgeBase, 0)
	for rows.Next() {
		kb, err := scanKnowledgeBase(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan knowledge base: %w", err)
		}
		kbs = append(kbs, kb)
	}

	return kbs, total, nil
}

// Update 更新知识库
func (r *KnowledgeBaseRepository) Update(ctx context.Context, kb *model.KnowledgeBase) error {
	query := `
		UPDATE knowledge_bases
		SET name = ?, display_name = ?, description = ?, type = ?, config = ?, query_endpoint = ?, status = ?, updated_at = NOW()
		WHERE id = ?
	`
	config, _ := json.Marshal(kb.Config)

	_, err := r.db.ExecContext(ctx, query,
		kb.Name, kb.DisplayName, kb.Description, kb.Type, config, kb.QueryEndpoint, kb.Status, kb.ID.String(),
	)
	return err
}

// Delete 删除知识库
func (r *KnowledgeBaseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM knowledge_bases WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// UpdateQueryStats 更新查询统计
func (r *KnowledgeBaseRepository) UpdateQueryStats(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE knowledge_bases SET query_count = query_count + 1, last_query_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id.String())
	return err
}

// FindByStatus 根据状态查找知识库
func (r *KnowledgeBaseRepository) FindByStatus(ctx context.Context, status model.KnowledgeBaseStatus) ([]*model.KnowledgeBase, error) {
	query := `
		SELECT id, name, display_name, description, type, config, query_endpoint, status, last_query_at, query_count, created_at, updated_at
		FROM knowledge_bases WHERE status = ?
	`
	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to find knowledge bases by status: %w", err)
	}
	defer rows.Close()

	kbs := make([]*model.KnowledgeBase, 0)
	for rows.Next() {
		kb, err := scanKnowledgeBase(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan knowledge base: %w", err)
		}
		kbs = append(kbs, kb)
	}

	return kbs, nil
}