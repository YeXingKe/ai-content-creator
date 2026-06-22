

type ArticleHandler struct {
    svc        *service.ArticleService
    userSvc    *service.UserService
    sseManager *common.SSEManager
}

func (h *ArticleHandler) Create(c *gin.Context) {
    var req model.CreateArticleRequest
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

    taskID, err := h.svc.Create(user, &req)
    if err != nil {
        handleError(c, err)
        return
    }

    c.JSON(http.StatusOK, common.Success(taskID))
}

func (h *ArticleHandler) GetProgress(c *gin.Context) {
    taskID := c.Param("taskId")
    // ... 权限校验 ...

    // 设置 SSE 响应头
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")

    messageChan := h.sseManager.Register(taskID)
    defer h.sseManager.Unregister(taskID)

    c.Stream(func(w io.Writer) bool {
        select {
        case msg, ok := <-messageChan:
            if !ok {
                return false
            }
            c.SSEvent("message", msg)
            c.Writer.Flush()
            return true
        case <-c.Request.Context().Done():
            return false
        case <-time.After(30 * time.Minute):
            return false
        }
    })
}

func (h *ArticleHandler) Get(c *gin.Context) {
    taskID := c.Param("taskId")
    session := sessions.Default(c)
    user, err := h.userSvc.GetLoginUser(session)
    if err != nil {
        handleError(c, err)
        return
    }
    isAdmin := user.UserRole == common.AdminRole
    article, err := h.svc.GetByTaskID(taskID, user.ID, isAdmin)
    if err != nil {
        handleError(c, err)
        return
    }
    c.JSON(http.StatusOK, common.Success(article))
}

func (h *ArticleHandler) List(c *gin.Context) {
    var req model.QueryArticleRequest
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
    page, err := h.svc.ListByPage(&req, user.ID, isAdmin)
    if err != nil {
        handleError(c, err)
        return
    }
    c.JSON(http.StatusOK, common.Success(page))
}

func (h *ArticleHandler) Delete(c *gin.Context) {
    var req model.DeleteRequest
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
    if err := h.svc.Delete(req.ID, user.ID, isAdmin); err != nil {
        handleError(c, err)
        return
    }
    c.JSON(http.StatusOK, common.Success(true))
}
