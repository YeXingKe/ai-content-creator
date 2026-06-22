

func (s *ArticleService) Create(user *model.User, req *model.CreateArticleRequest) (string, error) {
    if req.Topic == "" {
        return "", common.ErrParams.WithMessage("选题不能为空")
    }

    // 检查并消耗配额（原子操作）
    if err := s.quotaSvc.CheckAndConsumeQuota(user); err != nil {
        return "", err
    }

    taskID := uuid.NewString()
    article := &model.Article{
        TaskID: taskID, UserID: user.ID,
        Topic: req.Topic, Status: model.StatusPending,
        CreateTime: time.Now(),
    }
    if err := s.store.Create(article); err != nil {
        return "", common.ErrOperation
    }

    go s.executeAsync(taskID, req.Topic)
    return taskID, nil
}

func (s *ArticleService) executeAsync(taskID, topic string) {
    _ = s.store.UpdateStatus(taskID, model.StatusProcessing, nil)

    state := &model.ArticleState{TaskID: taskID, Topic: topic}
    err := s.agentSvc.Execute(context.Background(), state)

    if err != nil {
        errMsg := err.Error()
        _ = s.store.UpdateStatus(taskID, model.StatusFailed, &errMsg)
        s.sseManager.Send(taskID, map[string]interface{}{
            "type": "ERROR", "message": errMsg,
        })
        s.sseManager.Complete(taskID)
        return
    }

    if err := s.saveArticle(taskID, state); err != nil {
        errMsg := "保存文章失败"
        _ = s.store.UpdateStatus(taskID, model.StatusFailed, &errMsg)
        return
    }

    _ = s.store.UpdateStatus(taskID, model.StatusCompleted, nil)
    s.sseManager.Send(taskID, map[string]interface{}{
        "type": "ALL_COMPLETE", "taskId": taskID,
    })
    s.sseManager.Complete(taskID)
}

func (s *ArticleService) saveArticle(taskID string, state *model.ArticleState) error {
    article, err := s.store.GetByTaskID(taskID)
    if err != nil {
        return err
    }

    outlineJSON, _ := json.Marshal(state.Outline.Sections)
    imagesJSON, _ := json.Marshal(state.Images)
    outlineStr := string(outlineJSON)
    imagesStr := string(imagesJSON)
    now := time.Now()

    article.MainTitle = &state.Title.MainTitle
    article.SubTitle = &state.Title.SubTitle
    article.Outline = &outlineStr
    article.Content = &state.Content
    article.FullContent = &state.FullContent
    article.Images = &imagesStr
    article.CompletedTime = &now

    return s.store.Update(article)
}


func (s *ArticleService) GetByTaskID(taskID string, userID int64, isAdmin bool) (*model.ArticleInfo, error) {
    article, err := s.store.GetByTaskID(taskID)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, common.ErrNotFound.WithMessage("文章不存在")
        }
        return nil, common.ErrSystem
    }

    // 权限校验：只能查看自己的文章（管理员除外）
    if !isAdmin && article.UserID != userID {
        return nil, common.ErrNoAuth
    }

    return article.ToArticleInfo(), nil
}

func (s *ArticleService) ListByPage(req *model.QueryArticleRequest,
    userID int64, isAdmin bool) (*model.PageResult, error) {
    if req.PageNum <= 0 { req.PageNum = common.DefaultPageNum }
    if req.PageSize <= 0 { req.PageSize = common.DefaultPageSize }
    if req.PageSize > common.MaxPageSize { req.PageSize = common.MaxPageSize }

    queryUserID := &userID
    if isAdmin && req.UserID != nil {
        queryUserID = req.UserID
    }

    articles, total, err := s.store.List(queryUserID, req.Status, isAdmin, req.PageNum, req.PageSize)
    if err != nil {
        return nil, common.ErrSystem
    }

    articleInfos := make([]model.ArticleInfo, 0, len(articles))
    for i := range articles {
        if info := articles[i].ToArticleInfo(); info != nil {
            articleInfos = append(articleInfos, *info)
        }
    }

    return &model.PageResult{
        Total: total, Records: articleInfos,
        PageNum: req.PageNum, PageSize: req.PageSize,
    }, nil
}

func (s *ArticleService) Delete(id int64, userID int64, isAdmin bool) error {
    article, err := s.store.GetByID(id)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return common.ErrNotFound
        }
        return common.ErrSystem
    }

    if !isAdmin && article.UserID != userID {
        return common.ErrNoAuth
    }

    if err := s.store.Delete(id); err != nil {
        return common.ErrOperation
    }
    return nil
}


