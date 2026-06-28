// 文章模块 HTTP 入口层：解析请求 → 鉴权 → 调用 Service → 返回 JSON 或 SSE 流。
// 业务逻辑在 service/article.go，本文件不直接访问数据库。
package handler

import (
	"io"        // GetProgress 中 c.Stream 回调需要 io.Writer
	"net/http"  // HTTP 状态码常量（StatusOK、StatusBadRequest 等）
	"time"      // SSE 长连接超时控制

	"github.com/gin-contrib/sessions"                      // 从 Cookie/Redis 读取 Session，获取当前登录用户
	"github.com/gin-gonic/gin"                               // Web 框架，gin.Context 承载请求与响应
	"github.com/ai-content-creator/backend/internal/common"  // 统一响应格式、错误码、SSE 管理器、风格校验
	"github.com/ai-content-creator/backend/internal/model"   // 请求/响应 DTO（CreateArticleRequest 等）
	"github.com/ai-content-creator/backend/internal/service" // 业务层，Handler 通过它访问 Store 与 Agent
)

// ArticleHandler 文章处理器，依赖注入各 Service 与 SSE 管理器
type ArticleHandler struct {
	svc             *service.ArticleService   // 文章 CRUD、创建任务、确认标题/大纲、AI 改大纲
	userSvc         *service.UserService      // 从 Session 解析当前登录用户
	agentLogService *service.AgentLogService  // 查询智能体执行日志（GetExecutionLogs）
	sseManager      *common.SSEManager        // 注册/注销 SSE 连接，推送生成进度
}

// NewArticleHandler 创建文章处理器（在 app.go 中完成依赖注入）
func NewArticleHandler(svc *service.ArticleService, userSvc *service.UserService, agentLogService *service.AgentLogService, sseManager *common.SSEManager) *ArticleHandler {
	return &ArticleHandler{
		svc:             svc,             // 注入文章业务服务
		userSvc:         userSvc,         // 注入用户服务（鉴权用）
		agentLogService: agentLogService, // 注入 Agent 日志服务
		sseManager:      sseManager,      // 注入 SSE 连接管理器
	}
}

// Create 创建文章任务 POST /api/article/create
// @Summary      创建文章任务
// @Description  创建 AI 文章生成任务，扣减配额后异步生成标题方案。返回 taskId，前端通过 SSE 订阅进度。
// @Tags         文章
// @Accept       json
// @Produce      json
// @Param        body  body      model.CreateArticleRequest  true  "创建参数"
// @Success      200   {object}  common.BaseResponse{data=string}  "data 为 taskId（UUID）"
// @Failure      200   {object}  common.BaseResponse  "业务错误（code 非 0，如参数错误、未登录、配额不足）"
// @Security     SessionCookie
// @Router       /article/create [post]
func (h *ArticleHandler) Create(c *gin.Context) {
	var req model.CreateArticleRequest // 声明请求体结构体（topic 必填，style/enabledImageMethods 可选）
	if err := c.ShouldBindJSON(&req); err != nil { // 将 JSON body 绑定到 req，失败说明参数缺失或类型错误
		c.JSON(http.StatusOK, common.Error(common.ErrParams)) // 业务错误仍返回 HTTP 200，错误码在 JSON 的 code 字段
		return                                                // 终止处理，不再往下执行
	}

	if !common.IsValidArticleStyle(req.Style) { // 校验风格是否为 tech/emotional/educational/humorous，空字符串合法
		c.JSON(http.StatusOK, common.Error(common.ErrParams.WithMessage("无效的文章风格")))
		return
	}

	session := sessions.Default(c)                    // 获取当前请求的 Session（Redis + Cookie）
	user, err := h.userSvc.GetLoginUser(session)      // 从 Session 读 userLoginState，再查库得到 User
	if err != nil {                                 // 未登录或 Session 无效
		handleError(c, err) // 统一错误处理：AppError 原样返回，否则返回系统错误
		return
	}

	taskID, err := h.svc.Create(user, &req) // Service 内：扣配额、写 article 表、异步 Phase1 生成标题，返回 UUID
	if err != nil {                         // 配额不足、配图方式无权限等
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(taskID)) // 成功：{ code: 0, data: "task-uuid" }
}

// GetProgress SSE 推送生成进度 GET /api/article/progress/:taskId（路由未挂 userAuth，在此 Handler 内鉴权）
// @Summary      SSE 推送文章生成进度
// @Description  建立 Server-Sent Events 长连接，实时推送生成进度。需登录且有权访问该任务。响应 Content-Type 为 text/event-stream，事件名 message，data 为 JSON 字符串。
// @Tags         文章
// @Produce      text/event-stream
// @Param        taskId  path  string  true  "任务 ID（UUID）"
// @Success      200  {string}  string  "SSE 事件流"
// @Failure      400  {object}  common.BaseResponse  "参数错误"
// @Failure      401  {object}  common.BaseResponse  "未登录"
// @Failure      403  {object}  common.BaseResponse  "无权访问该任务"
// @Security     SessionCookie
// @Router       /article/progress/{taskId} [get]
func (h *ArticleHandler) GetProgress(c *gin.Context) {
	taskID := c.Param("taskId") // 从路径参数读取任务 ID
	if taskID == "" {
		c.JSON(http.StatusBadRequest, common.Error(common.ErrParams.WithMessage("任务ID不能为空"))) // SSE 接口用标准 HTTP 401/403/400
		return
	}

	session := sessions.Default(c)
	user, err := h.userSvc.GetLoginUser(session) // 必须登录才能订阅进度
	if err != nil {
		c.JSON(http.StatusUnauthorized, common.Error(common.ErrNotLogin)) // 401 未授权
		return
	}

	isAdmin := user.UserRole == common.AdminRole              // 管理员可订阅任意用户的任务进度
	_, err = h.svc.GetByTaskID(taskID, user.ID, isAdmin)      // 校验任务存在且当前用户有权访问（非本人且非管理员则拒绝）
	if err != nil {
		c.JSON(http.StatusForbidden, common.Error(err.(*common.AppError))) // 403 禁止访问
		return
	}

	c.Header("Content-Type", "text/event-stream")              // 声明 SSE 协议
	c.Header("Cache-Control", "no-cache")                      // 禁止缓存流式响应
	c.Header("Connection", "keep-alive")                       // 保持长连接
	c.Header("X-Accel-Buffering", "no")                        // 禁用 Nginx 缓冲，否则客户端收不到实时推送
	c.Header("Access-Control-Allow-Origin", "http://localhost:5173") // 允许前端 dev 服务器跨域
	c.Header("Access-Control-Allow-Credentials", "true")       // SSE 需携带 Cookie（Session）

	messageChan := h.sseManager.Register(taskID) // 为该 taskId 创建缓冲 channel（容量 100），后台 Agent 通过 Send 写入
	defer h.sseManager.Unregister(taskID)        // 函数结束时关闭 channel 并从 map 删除，防止泄漏

	c.Stream(func(w io.Writer) bool { // Gin 长连接循环；返回 true 继续，false 结束流
		select {
		case msg, ok := <-messageChan: // 等待后台推送的消息（JSON 字符串）
			if !ok { // channel 已关闭（Unregister 触发）
				return false // 结束 SSE 流
			}
			c.SSEvent("message", msg) // 写入 SSE 帧：event: message\ndata: {...}\n\n
			c.Writer.Flush()          // 立即刷到客户端，避免缓冲延迟
			return true               // 继续下一轮循环
		case <-c.Request.Context().Done(): // 客户端断开（关 tab、网络中断）
			return false
		case <-time.After(30 * time.Minute): // 最长保持 30 分钟，防止僵尸连接
			return false
		}
	})
}

// Get 获取文章详情 GET /api/article/:taskId
// @Summary      获取文章详情
// @Description  根据 taskId 获取文章详情，含标题、大纲、正文、状态、阶段等。普通用户只能查看自己的文章，管理员可查看全部。
// @Tags         文章
// @Produce      json
// @Param        taskId  path  string  true  "任务 ID（UUID）"
// @Success      200  {object}  common.BaseResponse{data=model.ArticleInfo}
// @Failure      200  {object}  common.BaseResponse  "业务错误（未登录、无权限、任务不存在）"
// @Security     SessionCookie
// @Router       /article/{taskId} [get]
func (h *ArticleHandler) Get(c *gin.Context) {
	taskID := c.Param("taskId") // 路径参数 taskId
	if taskID == "" {
		c.JSON(http.StatusOK, common.Error(common.ErrParams.WithMessage("任务ID不能为空")))
		return
	}

	session := sessions.Default(c)
	user, err := h.userSvc.GetLoginUser(session)
	if err != nil {
		handleError(c, err)
		return
	}

	isAdmin := user.UserRole == common.AdminRole // 管理员可查看任意用户文章

	article, err := h.svc.GetByTaskID(taskID, user.ID, isAdmin) // 返回 ArticleInfo（JSON 字段已解析为结构体）
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(article)) // 返回文章详情（标题、大纲、正文、状态、阶段等）
}

// List 分页查询文章列表 POST /api/article/list
// @Summary      分页查询文章列表
// @Description  分页查询文章列表。非管理员只能查看自己的文章；管理员可按 userId 筛选。
// @Tags         文章
// @Accept       json
// @Produce      json
// @Param        body  body      model.QueryArticleRequest  true  "查询参数"
// @Success      200   {object}  common.BaseResponse{data=model.ArticlePageResult}
// @Failure      200   {object}  common.BaseResponse  "业务错误"
// @Security     SessionCookie
// @Router       /article/list [post]
func (h *ArticleHandler) List(c *gin.Context) {
	var req model.QueryArticleRequest // 含 userId、status、pageNum、pageSize
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	session := sessions.Default(c)
	user, err := h.userSvc.GetLoginUser(session)
	if err != nil {
		handleError(c, err)
		return
	}

	isAdmin := user.UserRole == common.AdminRole

	page, err := h.svc.ListByPage(&req, user.ID, isAdmin) // 非管理员强制只看自己的；管理员可按 userId 筛选
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(page)) // PageResult：records + total 等
}

// Delete 删除文章（逻辑删除） POST /api/article/delete
// @Summary      删除文章
// @Description  逻辑删除文章（isDelete=1）。普通用户只能删除自己的文章，管理员可删除任意文章。
// @Tags         文章
// @Accept       json
// @Produce      json
// @Param        body  body      model.DeleteRequest  true  "删除参数（文章主键 id，非 taskId）"
// @Success      200   {object}  common.BaseResponse{data=bool}
// @Failure      200   {object}  common.BaseResponse  "业务错误"
// @Security     SessionCookie
// @Router       /article/delete [post]
func (h *ArticleHandler) Delete(c *gin.Context) {
	var req model.DeleteRequest // 含文章主键 id（非 taskId）
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	session := sessions.Default(c)
	user, err := h.userSvc.GetLoginUser(session)
	if err != nil {
		handleError(c, err)
		return
	}

	isAdmin := user.UserRole == common.AdminRole

	if err := h.svc.Delete(req.ID, user.ID, isAdmin); err != nil { // Service 校验归属后 isDelete=1
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(true))
}

// ConfirmTitle 确认标题并输入补充描述 POST /api/article/confirm-title
// @Summary      确认标题
// @Description  用户从标题方案中选择主副标题，可选填写补充描述。确认后异步进入 Phase2 生成大纲，进度通过 SSE 推送。
// @Tags         文章
// @Accept       json
// @Produce      json
// @Param        body  body      model.ConfirmTitleRequest  true  "确认标题参数"
// @Success      200   {object}  common.BaseResponse
// @Failure      200   {object}  common.BaseResponse  "业务错误"
// @Security     SessionCookie
// @Router       /article/confirm-title [post]
func (h *ArticleHandler) ConfirmTitle(c *gin.Context) {
	var req model.ConfirmTitleRequest // taskId、selectedMainTitle、selectedSubTitle、userDescription（可选）
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	session := sessions.Default(c)
	user, err := h.userSvc.GetLoginUser(session)
	if err != nil {
		handleError(c, err)
		return
	}

	isAdmin := user.UserRole == common.AdminRole

	if err := h.svc.ConfirmTitle(req.TaskID, req.SelectedMainTitle, req.SelectedSubTitle, req.UserDescription, user.ID, isAdmin); err != nil {
		// Service 持久化选定标题，更新 phase，异步触发 Phase2 生成大纲
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(nil)) // 无返回体，前端通过 SSE 收进度
}

// ConfirmOutline 确认大纲 POST /api/article/confirm-outline
// @Summary      确认大纲
// @Description  用户确认或编辑后的大纲，确认后异步进入 Phase3 生成正文与配图，进度通过 SSE 推送。
// @Tags         文章
// @Accept       json
// @Produce      json
// @Param        body  body      model.ConfirmOutlineRequest  true  "确认大纲参数"
// @Success      200   {object}  common.BaseResponse
// @Failure      200   {object}  common.BaseResponse  "业务错误"
// @Security     SessionCookie
// @Router       /article/confirm-outline [post]
func (h *ArticleHandler) ConfirmOutline(c *gin.Context) {
	var req model.ConfirmOutlineRequest // taskId + outline（[]OutlineSection）
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	session := sessions.Default(c)
	user, err := h.userSvc.GetLoginUser(session)
	if err != nil {
		handleError(c, err)
		return
	}

	isAdmin := user.UserRole == common.AdminRole

	if err := h.svc.ConfirmOutline(req.TaskID, req.Outline, user.ID, isAdmin); err != nil {
		// Service 持久化大纲，异步触发 Phase3 生成正文与配图
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(nil))
}

// AiModifyOutline AI 根据用户建议修改大纲 POST /api/article/ai-modify-outline
// @Summary      AI 修改大纲
// @Description  根据用户自然语言修改意见，调用 LLM 重新生成大纲。返回修改后的大纲，需再调用 confirm-outline 确认。
// @Tags         文章
// @Accept       json
// @Produce      json
// @Param        body  body      model.AiModifyOutlineRequest  true  "修改建议"
// @Success      200   {object}  common.BaseResponse{data=[]model.OutlineSection}
// @Failure      200   {object}  common.BaseResponse  "业务错误"
// @Security     SessionCookie
// @Router       /article/ai-modify-outline [post]
func (h *ArticleHandler) AiModifyOutline(c *gin.Context) {
	var req model.AiModifyOutlineRequest // taskId + modifySuggestion（自然语言修改意见）
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	session := sessions.Default(c)
	user, err := h.userSvc.GetLoginUser(session)
	if err != nil {
		handleError(c, err)
		return
	}

	isAdmin := user.UserRole == common.AdminRole

	modifiedOutline, err := h.svc.AiModifyOutline(req.TaskID, req.ModifySuggestion, user, isAdmin) // 调用 LLM 改大纲，不自动确认
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(modifiedOutline)) // 返回修改后的大纲，前端需再调 ConfirmOutline 确认
}

// GetExecutionLogs 获取任务智能体执行日志 GET /api/article/execution-logs/:taskId（当前未做登录/归属校验）
// @Summary      获取智能体执行日志
// @Description  获取指定任务的智能体执行统计与各 Agent 详细日志（耗时、状态、Prompt 等）。
// @Tags         文章
// @Produce      json
// @Param        taskId  path  string  true  "任务 ID（UUID）"
// @Success      200  {object}  common.BaseResponse{data=model.AgentExecutionStats}
// @Failure      200  {object}  common.BaseResponse  "业务错误"
// @Router       /article/execution-logs/{taskId} [get]
func (h *ArticleHandler) GetExecutionLogs(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusOK, common.Error(common.ErrParams.WithMessage("任务ID不能为空")))
		return
	}

	stats, err := h.agentLogService.GetExecutionStats(taskID) // 聚合各 Agent 的执行时间、状态等
	if err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrSystem.WithMessage("获取执行日志失败: "+err.Error())))
		return
	}

	c.JSON(http.StatusOK, common.Success(stats))
}
