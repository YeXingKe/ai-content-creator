package model

import "time"

// Article 文章实体
type Article struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement" json:"id"`                                      // 主键 ID
	TaskID              string     `gorm:"column:taskId;uniqueIndex:uk_taskId" json:"taskId"`                       // 任务 ID（UUID，唯一标识一次生成任务）
	UserID              int64      `gorm:"column:userId;index:idx_userId" json:"userId"`                              // 所属用户 ID
	Topic               string     `gorm:"column:topic" json:"topic"`                                                 // 文章主题/关键词
	UserDescription     *string    `gorm:"column:userDescription;type:text" json:"userDescription"`                   // 用户补充描述（可选）
	MainTitle           *string    `gorm:"column:mainTitle" json:"mainTitle"`                                         // 主标题（用户选定后写入）
	SubTitle            *string    `gorm:"column:subTitle" json:"subTitle"`                                           // 副标题（用户选定后写入）
	TitleOptions        *string    `gorm:"column:titleOptions;type:json" json:"titleOptions"`                         // 标题方案列表（JSON 字符串）
	Outline             *string    `gorm:"column:outline;type:json" json:"outline"`                                   // 大纲结构（JSON 字符串）
	Content             *string    `gorm:"column:content;type:text" json:"content"`                                   // 正文内容（不含配图占位符替换）
	FullContent         *string    `gorm:"column:fullContent;type:text" json:"fullContent"`                             // 完整正文（含图片 URL 替换后的 Markdown）
	Images              *string    `gorm:"column:images;type:json" json:"images"`                                     // 配图结果列表（JSON 字符串）
	Status              string     `gorm:"column:status;default:PENDING;index:idx_status" json:"status"`              // 任务状态：PENDING / PROCESSING / COMPLETED / FAILED
	Phase               string     `gorm:"column:phase;default:PENDING" json:"phase"`                                 // 当前阶段：PENDING / TITLE_GENERATING / TITLE_SELECTING / OUTLINE_GENERATING / OUTLINE_EDITING / CONTENT_GENERATING
	ErrorMessage        *string    `gorm:"column:errorMessage;type:text" json:"errorMessage"`                         // 失败时的错误信息
	Style               string     `gorm:"column:style" json:"style"`                                                 // 文章风格：tech / emotional / educational / humorous
	EnabledImageMethods *string    `gorm:"column:enabledImageMethods;type:json" json:"enabledImageMethods"`           // 允许的配图方式列表（JSON 字符串）
	CreateTime          time.Time  `gorm:"column:createTime;autoCreateTime;index:idx_createTime" json:"createTime"`   // 创建时间
	CompletedTime       *time.Time `gorm:"column:completedTime" json:"completedTime"`                                 // 完成时间（未完成时为 nil）
	UpdateTime          time.Time  `gorm:"column:updateTime;autoUpdateTime" json:"updateTime"`                        // 最后更新时间
	IsDelete            int        `gorm:"column:isDelete;default:0" json:"-"`                                        // 软删除标记：0 正常，1 已删除
}

func (Article) TableName() string {
	return "article"
}

// ArticleStatus 文章状态
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
)

// ArticlePhase 文章阶段
const (
	PhasePending           = "PENDING"            // 等待处理
	PhaseTitleGenerating   = "TITLE_GENERATING"   // 生成标题中
	PhaseTitleSelecting    = "TITLE_SELECTING"    // 等待选择标题
	PhaseOutlineGenerating = "OUTLINE_GENERATING" // 生成大纲中
	PhaseOutlineEditing    = "OUTLINE_EDITING"    // 等待编辑大纲
	PhaseContentGenerating = "CONTENT_GENERATING" // 生成正文中
)

// ArticleInfo 文章信息（API 响应）
type ArticleInfo struct {
	ID                  int64            `json:"id"`                  // 主键 ID
	TaskID              string           `json:"taskId"`              // 任务 ID
	UserID              int64            `json:"userId"`              // 所属用户 ID
	Topic               string           `json:"topic"`               // 文章主题
	UserDescription     *string          `json:"userDescription"`     // 用户补充描述
	MainTitle           *string          `json:"mainTitle"`           // 主标题
	SubTitle            *string          `json:"subTitle"`            // 副标题
	TitleOptions        []TitleOption    `json:"titleOptions"`        // 标题方案列表（已解析）
	Outline             []OutlineSection `json:"outline"`             // 大纲章节列表（已解析）
	Content             *string          `json:"content"`             // 正文内容
	FullContent         *string          `json:"fullContent"`         // 完整正文（含图片）
	Images              []ImageResult    `json:"images"`              // 配图结果列表（已解析）
	Status              string           `json:"status"`              // 任务状态
	Phase               string           `json:"phase"`               // 当前阶段
	ErrorMessage        *string          `json:"errorMessage"`        // 错误信息
	Style               string           `json:"style"`               // 文章风格
	EnabledImageMethods []string         `json:"enabledImageMethods"` // 允许的配图方式列表（已解析）
	CreateTime          time.Time        `json:"createTime"`          // 创建时间
	CompletedTime       *time.Time       `json:"completedTime"`       // 完成时间
}

// ToArticleInfo 转换为文章信息
func (a *Article) ToArticleInfo() *ArticleInfo {
	if a == nil {
		return nil
	}

	info := &ArticleInfo{
		ID:              a.ID,
		TaskID:          a.TaskID,
		UserID:          a.UserID,
		Topic:           a.Topic,
		UserDescription: a.UserDescription,
		MainTitle:       a.MainTitle,
		SubTitle:        a.SubTitle,
		Content:         a.Content,
		FullContent:     a.FullContent,
		Status:          a.Status,
		Phase:           a.Phase,
		ErrorMessage:    a.ErrorMessage,
		Style:           a.Style,
		CreateTime:      a.CreateTime,
		CompletedTime:   a.CompletedTime,
	}

	// 解析 JSON 字段
	if a.TitleOptions != nil {
		parseJSON(*a.TitleOptions, &info.TitleOptions)
	}
	if a.Outline != nil {
		parseJSON(*a.Outline, &info.Outline)
	}
	if a.Images != nil {
		parseJSON(*a.Images, &info.Images)
	}
	if a.EnabledImageMethods != nil && *a.EnabledImageMethods != "" {
		parseJSON(*a.EnabledImageMethods, &info.EnabledImageMethods)
	}

	return info
}

// ArticleState 文章生成状态（智能体编排过程中共享）
type ArticleState struct {
	TaskID                  string             `json:"taskId"`                  // 任务 ID
	Topic                   string             `json:"topic"`                   // 文章主题
	UserDescription         string             `json:"userDescription"`         // 用户补充描述
	Style                   string             `json:"style"`                   // 文章风格
	Phase                   string             `json:"phase"`                   // 当前阶段
	EnabledImageMethods     []string           `json:"enabledImageMethods"`     // 允许的配图方式列表
	TitleOptions            []TitleOption      `json:"titleOptions"`            // 标题方案列表
	Title                   *TitleResult       `json:"title"`                   // 用户选定的标题
	Outline                 *OutlineResult     `json:"outline"`                 // 大纲结果
	Content                 string             `json:"content"`                 // 正文内容
	ContentWithPlaceholders string             `json:"contentWithPlaceholders"` // 含 {{IMAGE_PLACEHOLDER_N}} 占位符的正文
	FullContent             string             `json:"fullContent"`             // 替换占位符后的完整正文
	ImageRequirements       []ImageRequirement `json:"imageRequirements"`       // 配图需求列表
	Images                  []ImageResult      `json:"images"`                  // 配图结果列表
}

// TitleOption 标题方案
type TitleOption struct {
	MainTitle string `json:"mainTitle"` // 主标题
	SubTitle  string `json:"subTitle"`  // 副标题
}

// TitleResult 标题结果
type TitleResult struct {
	MainTitle string `json:"mainTitle"` // 主标题
	SubTitle  string `json:"subTitle"`  // 副标题
}

// OutlineResult 大纲结果
type OutlineResult struct {
	Sections []OutlineSection `json:"sections"` // 章节列表
}

// OutlineSection 大纲章节
type OutlineSection struct {
	Section int      `json:"section"` // 章节序号
	Title   string   `json:"title"`   // 章节标题
	Points  []string `json:"points"`  // 章节要点列表
}

// ImageRequirement 配图需求
type ImageRequirement struct {
	Position      int    `json:"position"`      // 在正文中的位置序号
	Type          string `json:"type"`          // 配图类型
	SectionTitle  string `json:"sectionTitle"`  // 所属章节标题
	ImageSource   string `json:"imageSource"`   // 配图来源：PEXELS / NANO_BANANA / MERMAID / ICONIFY / EMOJI_PACK / SVG_DIAGRAM
	Keywords      string `json:"keywords"`      // 搜索关键词（Pexels 等）
	Prompt        string `json:"prompt"`        // AI 生图 Prompt
	PlaceholderID string `json:"placeholderId"` // 占位符 ID，如 {{IMAGE_PLACEHOLDER_1}}
}

// ImageResult 配图结果
type ImageResult struct {
	Position      int    `json:"position"`      // 在正文中的位置序号
	URL           string `json:"url"`           // 图片访问 URL
	Method        string `json:"method"`        // 实际使用的配图方式
	Keywords      string `json:"keywords"`      // 搜索关键词
	SectionTitle  string `json:"sectionTitle"`  // 所属章节标题
	Description   string `json:"description"`   // 图片描述
	PlaceholderID string `json:"placeholderId"` // 对应的占位符 ID
}
