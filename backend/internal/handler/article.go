

type ArticleHandler struct {
    svc        *service.ArticleService
    userSvc    *service.UserService
    sseManager *common.SSEManager
}

func (h *ArticleHandler) Create(c *gin.Context) {
    var req model.CreateArticleRequest  // 声明请求体变量
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusOK, common.Error(common.ErrParams)) // 参数错误
        return
    }

    session := sessions.Default(c) // 获取当前会话
    user, err := h.userSvc.GetLoginUser(session) // 获取当前登录用户
    if err != nil {
        handleError(c, err) // 统一错误处理
        return
    }

    taskID, err := h.svc.Create(user, &req) // 创建文章
    if err != nil {
        handleError(c, err) // 统一错误处理
        return
    }

    c.JSON(http.StatusOK, common.Success(taskID)) // 返回成功响应
}

func (h *ArticleHandler) GetProgress(c *gin.Context) { // 获取文章进度
    taskID := c.Param("taskId")
    // ... 权限校验 ...

    // 设置 SSE 响应头
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")

    messageChan := h.sseManager.Register(taskID) // 注册 SSE 通道
    defer h.sseManager.Unregister(taskID) // 注销 SSE 通道

    // 数据从 channel 流出来 → <-channel
    // 流式输出：每次从 channel 中获取一条消息，然后发送给客户端
    // <-ch 从 channel ch 接收（读）
    // ch <- x 向 channel ch 发送（写）
    
    c.Stream(func(w io.Writer) bool { // 流式输出
        select {
        case msg, ok := <-messageChan: // 获取消息
            if !ok {
                return false
            }
            c.SSEvent("message", msg) // 发送消息
            c.Writer.Flush() // 刷新缓冲区
            return true
        case <-c.Request.Context().Done(): // 请求上下文    完成
            return false
        case <-time.After(30 * time.Minute): // 超时
            return false
        }
    })
}

func (h *ArticleHandler) Get(c *gin.Context) {
    taskID := c.Param("taskId") // 获取任务 ID
    session := sessions.Default(c) // 获取当前会话
    user, err := h.userSvc.GetLoginUser(session) // 获取当前登录用户
    if err != nil {
        handleError(c, err) // 统一错误处理
        return
    }
    isAdmin := user.UserRole == common.AdminRole
    article, err := h.svc.GetByTaskID(taskID, user.ID, isAdmin) // 获取文章
    if err != nil {
        handleError(c, err)
        return
    }
    c.JSON(http.StatusOK, common.Success(article))
}

func (h *ArticleHandler) List(c *gin.Context) {
    var req model.QueryArticleRequest
    if err := c.ShouldBindJSON(&req); err != nil { // 绑定请求体
        c.JSON(http.StatusOK, common.Error(common.ErrParams)) // 参数错误
        return
    }
    session := sessions.Default(c) // 获取当前会话
    user, err := h.userSvc.GetLoginUser(session) // 获取当前登录用户
    if err != nil {
        handleError(c, err)
        return
    }
    isAdmin := user.UserRole == common.AdminRole
    page, err := h.svc.ListByPage(&req, user.ID, isAdmin)
    if err != nil {
        handleError(c, err)
        return
    }
    c.JSON(http.StatusOK, common.Success(page))
}

func (h *ArticleHandler) Delete(c *gin.Context) {
    var req model.DeleteRequest // 声明请求体变量
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusOK, common.Error(common.ErrParams)) // 参数错误
        return
    }
    session := sessions.Default(c) // 获取当前会话
    user, err := h.userSvc.GetLoginUser(session) // 获取当前登录用户
    if err != nil {
        handleError(c, err) // 统一错误处理
        return
    }
    isAdmin := user.UserRole == common.AdminRole
    if err := h.svc.Delete(req.ID, user.ID, isAdmin); err != nil {
        handleError(c, err) // 统一错误处理
        return
    }
    c.JSON(http.StatusOK, common.Success(true)) // 返回成功响应
}
