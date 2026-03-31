package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AssetPackageRepository 资产包数据访问
type AssetPackageRepository struct {
	db *sql.DB
}

// NewAssetPackageRepository 创建AssetPackage Repository
func NewAssetPackageRepository(db *sql.DB) *AssetPackageRepository {
	return &AssetPackageRepository{db: db}
}

// Create 创建AssetPackage
func (r *AssetPackageRepository) Create(ctx context.Context, pkg *model.AssetPackage) error {
	query := `
		INSERT INTO asset_packages (id, name, version, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		pkg.ID.String(), pkg.Name, pkg.Version, pkg.Description, pkg.CreatedAt, pkg.UpdatedAt,
	)
	return err
}

// scanAssetPackage 辅助函数，扫描AssetPackage行
func scanAssetPackage(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.AssetPackage, error) {
	pkg := &model.AssetPackage{}
	var idStr string
	var description sql.NullString

	err := scanner.Scan(
		&idStr, &pkg.Name, &pkg.Version, &description, &pkg.CreatedAt, &pkg.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	pkg.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		pkg.Description = description.String
	}

	return pkg, nil
}

// FindByID 根据ID查找
func (r *AssetPackageRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AssetPackage, error) {
	query := `
		SELECT id, name, version, description, created_at, updated_at
		FROM asset_packages WHERE id = ?
	`
	pkg, err := scanAssetPackage(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("asset package not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find asset package: %w", err)
	}
	return pkg, nil
}

// FindByName 根据名称查找
func (r *AssetPackageRepository) FindByName(ctx context.Context, name string) (*model.AssetPackage, error) {
	query := `
		SELECT id, name, version, description, created_at, updated_at
		FROM asset_packages WHERE name = ?
	`
	pkg, err := scanAssetPackage(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("asset package not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find asset package: %w", err)
	}
	return pkg, nil
}

// FindByNameAndVersion 根据名称和版本查找
func (r *AssetPackageRepository) FindByNameAndVersion(ctx context.Context, name, version string) (*model.AssetPackage, error) {
	query := `
		SELECT id, name, version, description, created_at, updated_at
		FROM asset_packages WHERE name = ? AND version = ?
	`
	pkg, err := scanAssetPackage(r.db.QueryRowContext(ctx, query, name, version))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("asset package not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find asset package: %w", err)
	}
	return pkg, nil
}

// List 列出AssetPackages，支持分页和搜索
func (r *AssetPackageRepository) List(ctx context.Context, query *model.AssetPackageListQuery) ([]*model.AssetPackage, int64, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM asset_packages " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count asset packages: %w", err)
	}

	// 分页
	page := query.Page
	pageSize := query.PageSize
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
		SELECT id, name, version, description, created_at, updated_at
		FROM asset_packages ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list asset packages: %w", err)
	}
	defer rows.Close()

	packages := make([]*model.AssetPackage, 0)
	for rows.Next() {
		pkg, err := scanAssetPackage(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan asset package: %w", err)
		}
		packages = append(packages, pkg)
	}

	return packages, total, nil
}

// Update 更新AssetPackage
func (r *AssetPackageRepository) Update(ctx context.Context, pkg *model.AssetPackage) error {
	query := `
		UPDATE asset_packages
		SET name = ?, version = ?, description = ?, updated_at = NOW()
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		pkg.Name, pkg.Version, pkg.Description, pkg.ID.String(),
	)
	return err
}

// Delete 删除AssetPackage
func (r *AssetPackageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM asset_packages WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}