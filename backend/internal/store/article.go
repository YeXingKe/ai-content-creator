package store

type ArticleStore struct {
    db *gorm.DB
}

func NewArticleStore(db *gorm.DB) *ArticleStore {
    return &ArticleStore{db: db}
}


func (s *ArticleStore) Create(article *model.Article) error {
    return s.db.Create(article).Error
}

func (s *ArticleStore) GetByTaskID(taskID string) (*model.Article, error) {
    var article model.Article
    err := s.db.Scopes(NotDeleted).Where("taskId = ?", taskID).First(&article).Error
    if err != nil {
        return nil, err
    }
    return &article, nil
}


func (s *ArticleStore) UpdateStatus(taskID, status string, errorMsg *string) error {
    updates := map[string]interface{}{"status": status}
    if errorMsg != nil {
        updates["errorMessage"] = *errorMsg
    }
    return s.db.Model(&model.Article{}).Where("taskId = ?", taskID).Updates(updates).Error
}

func (s *ArticleStore) List(userID *int64, status *string, isAdmin bool,
    pageNum, pageSize int64) ([]model.Article, int64, error) {

    var articles []model.Article
    var total int64
    query := s.db.Scopes(NotDeleted)

    if !isAdmin && userID != nil {
        query = query.Where("userId = ?", *userID)
    } else if userID != nil {
        query = query.Where("userId = ?", *userID)
    }

    if status != nil && *status != "" {
        query = query.Where("status = ?", *status)
    }

    query.Model(&model.Article{}).Count(&total)

    offset := (pageNum - 1) * pageSize
    query.Order("createTime DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&articles)

    return articles, total, nil
}
