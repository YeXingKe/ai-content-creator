package model

// CreateArticleRequest 创建文章请求
type CreateArticleRequest struct {
	Topic               string   `json:"topic" binding:"required" example:"AI 如何改变教育"`                                    // 文章主题/关键词（必填）
	Style               string   `json:"style" example:"tech" enums:"tech,emotional,educational,humorous"`                  // 文章风格，可为空则使用默认
	EnabledImageMethods []string `json:"enabledImageMethods" example:"PEXELS,MERMAID"`                                      // 允许的配图方式，为空表示支持全部
}

// QueryArticleRequest 查询文章请求
type QueryArticleRequest struct {
	UserID   *int64  `json:"userId" example:"1"`                                              // 按用户 ID 筛选（管理员可用，可选）
	Status   *string `json:"status" example:"COMPLETED" enums:"PENDING,PROCESSING,COMPLETED,FAILED"` // 按任务状态筛选（可选）
	PageNum  int64   `json:"pageNum" example:"1"`                                             // 页码，从 1 开始
	PageSize int64   `json:"pageSize" example:"10"`                                           // 每页条数
}

// ConfirmTitleRequest 确认标题请求
type ConfirmTitleRequest struct {
	TaskID            string  `json:"taskId" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"` // 任务 ID（UUID）
	SelectedMainTitle string  `json:"selectedMainTitle" binding:"required" example:"AI 重塑未来教育"`                // 用户选定的主标题
	SelectedSubTitle  string  `json:"selectedSubTitle" binding:"required" example:"从技术到课堂的变革之路"`                // 用户选定的副标题
	UserDescription   *string `json:"userDescription" example:"面向 K12 教师，语气专业"`                                 // 用户补充描述（可选）
}

// ConfirmOutlineRequest 确认大纲请求
type ConfirmOutlineRequest struct {
	TaskID  string           `json:"taskId" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"` // 任务 ID（UUID）
	Outline []OutlineSection `json:"outline" binding:"required"`                                             // 确认后的大纲章节列表
}

// AiModifyOutlineRequest AI 修改大纲请求
type AiModifyOutlineRequest struct {
	TaskID           string `json:"taskId" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"` // 任务 ID（UUID）
	ModifySuggestion string `json:"modifySuggestion" binding:"required" example:"增加一节关于 AI 伦理的内容"`             // 用户对大纲的修改建议（自然语言）
}

// ArticlePageResult 文章分页结果
type ArticlePageResult struct {
	Total    int64         `json:"total" example:"100"`   // 符合条件的文章总数
	Records  []ArticleInfo `json:"records"`               // 当前页文章列表
	PageNum  int64         `json:"pageNum" example:"1"`   // 当前页码
	PageSize int64         `json:"pageSize" example:"10"` // 每页条数
}
