package store

import (
	"github.com/ai-content-creator/backend/internal/model"
	"gorm.io/gorm"
)

// ArticleStore 文章数据存储（封装 article 表的 GORM 操作）
type ArticleStore struct {
	db *gorm.DB // 数据库连接实例，由 app 初始化时注入
}

// NewArticleStore 创建文章存储
func NewArticleStore(db *gorm.DB) *ArticleStore {
	return &ArticleStore{db: db} // 依赖注入，便于测试与替换
}

// Create 创建文章记录
func (s *ArticleStore) Create(article *model.Article) error {
	return s.db.Create(article).Error // INSERT INTO article ...
}

// GetByTaskID 根据任务 ID 获取文章（排除已软删）
func (s *ArticleStore) GetByTaskID(taskID string) (*model.Article, error) {
	var article model.Article
	err := s.db.Scopes(NotDeleted).Where("taskId = ?", taskID).First(&article).Error // 按 taskId 查唯一记录
	if err != nil {
		return nil, err // 未找到时返回 gorm.ErrRecordNotFound
	}
	return &article, nil
}

// GetByID 根据主键 ID 获取文章（排除已软删）
func (s *ArticleStore) GetByID(id int64) (*model.Article, error) {
	var article model.Article
	err := s.db.Scopes(NotDeleted).Where("id = ?", id).First(&article).Error
	if err != nil {
		return nil, err
	}
	return &article, nil
}

// Update 按 taskId 更新文章字段（零值字段不会更新）
func (s *ArticleStore) Update(article *model.Article) error {
	return s.db.Scopes(NotDeleted).Where("taskId = ?", article.TaskID).Updates(article).Error
}

// UpdateStatus 更新文章状态与可选错误信息
func (s *ArticleStore) UpdateStatus(taskID, status string, errorMsg *string) error {
	updates := map[string]interface{}{
		"status": status, // 状态：PENDING / PROCESSING / COMPLETED / FAILED
	}
	if errorMsg != nil {
		updates["errorMessage"] = *errorMsg // 失败时写入错误原因
	}
	return s.db.Model(&model.Article{}).Where("taskId = ?", taskID).Updates(updates).Error
}

// Delete 删除文章（逻辑删除，isDelete 置 1）
func (s *ArticleStore) Delete(id int64) error {
	return s.db.Model(&model.Article{}).Where("id = ?", id).Update("isDelete", 1).Error
}

// List 分页查询文章列表
func (s *ArticleStore) List(userID *int64, status *string, isAdmin bool, pageNum, pageSize int64) ([]model.Article, int64, error) {
	var articles []model.Article // 当前页数据
	var total int64            // 符合条件的总条数

	query := s.db.Scopes(NotDeleted) // 基础查询：排除软删

	if !isAdmin && userID != nil {
		query = query.Where("userId = ?", *userID) // 普通用户只能查自己的文章
	} else if userID != nil {
		query = query.Where("userId = ?", *userID) // 管理员可按 userId 筛选
	}

	if status != nil && *status != "" {
		query = query.Where("status = ?", *status) // 按状态筛选（可选）
	}

	if err := query.Model(&model.Article{}).Count(&total).Error; err != nil { // 先统计总数
		return nil, 0, err
	}

	offset := (pageNum - 1) * pageSize // 计算偏移量
	if err := query.Order("createTime DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&articles).Error; err != nil {
		return nil, 0, err
	}

	return articles, total, nil
}

// UpdatePhase 更新文章当前阶段（如 TITLE_SELECTING、OUTLINE_EDITING）
func (s *ArticleStore) UpdatePhase(taskID, phase string) error {
	return s.db.Model(&model.Article{}).Where("taskId = ?", taskID).Update("phase", phase).Error
}

// UpdateTitleOptions 更新标题方案 JSON（Agent1 生成后持久化）
func (s *ArticleStore) UpdateTitleOptions(taskID, titleOptions string) error {
	return s.db.Model(&model.Article{}).Where("taskId = ?", taskID).Update("titleOptions", titleOptions).Error
}
