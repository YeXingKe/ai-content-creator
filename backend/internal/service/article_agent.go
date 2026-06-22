package service


// ArticleAgentService 文章智能体编排服务
type ArticleAgentService struct {
    llm        llms.Model
    pexels     *PexelsService
    cos        *CosService
    sseManager *common.SSEManager
}


func NewArticleAgentService(cfg *config.Config, pexels *PexelsService,
    cos *CosService, sseManager *common.SSEManager) (*ArticleAgentService, error) {

    llm, err := openai.New(
        openai.WithToken(cfg.AI.DashScope.APIKey),
        openai.WithModel(cfg.AI.DashScope.Model),
        openai.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
    )
    if err != nil {
        return nil, fmt.Errorf("create dashscope client: %w", err)
    }

    return &ArticleAgentService{
        llm: llm, pexels: pexels, cos: cos, sseManager: sseManager,
    }, nil
}

func (s *ArticleAgentService) Execute(ctx context.Context, state *model.ArticleState) error {
    // 智能体1：生成标题
    if err := s.agent1GenerateTitle(ctx, state); err != nil {
        return fmt.Errorf("agent1 failed: %w", err)
    }
    s.sendMessage(state.TaskID, map[string]interface{}{
        "type": "AGENT1_COMPLETE", "title": state.Title,
    })

    // 智能体2：生成大纲（流式）
    if err := s.agent2GenerateOutlineStream(ctx, state); err != nil {
        return fmt.Errorf("agent2 failed: %w", err)
    }
    s.sendMessage(state.TaskID, map[string]interface{}{
        "type": "AGENT2_COMPLETE", "outline": state.Outline.Sections,
    })

    // 智能体3：生成正文（流式）
    if err := s.agent3GenerateContent(ctx, state); err != nil {
        return fmt.Errorf("agent3 failed: %w", err)
    }
    s.sendMessage(state.TaskID, map[string]interface{}{"type": "AGENT3_COMPLETE"})

    // 智能体4：分析配图需求
    if err := s.agent4AnalyzeImageRequirements(ctx, state); err != nil {
        return fmt.Errorf("agent4 failed: %w", err)
    }
    s.sendMessage(state.TaskID, map[string]interface{}{
        "type": "AGENT4_COMPLETE", "imageRequirements": state.ImageRequirements,
    })

    // 智能体5：生成配图
    if err := s.agent5GenerateImages(ctx, state); err != nil {
        return fmt.Errorf("agent5 failed: %w", err)
    }
    s.sendMessage(state.TaskID, map[string]interface{}{
        "type": "AGENT5_COMPLETE", "images": state.Images,
    })

    // 图文合成
    s.mergeImagesIntoContent(state)
    s.sendMessage(state.TaskID, map[string]interface{}{
        "type": "MERGE_COMPLETE", "fullContent": state.FullContent,
    })

    return nil
}

func (s *ArticleAgentService) agent1GenerateTitle(ctx context.Context, state *model.ArticleState) error {
    prompt := strings.ReplaceAll(common.Agent1TitlePrompt, "{topic}", state.Topic)

    content, err := llms.GenerateFromSinglePrompt(ctx, s.llm, prompt)
    if err != nil {
        return fmt.Errorf("LLM call failed: %w", err)
    }

    var title model.TitleResult
    if err := json.Unmarshal([]byte(content), &title); err != nil {
        return fmt.Errorf("parse title failed: %w", err)
    }

    state.Title = &title
    log.Printf("智能体1：标题生成成功, mainTitle=%s", title.MainTitle)
    return nil
}

func (s *ArticleAgentService) agent2GenerateOutlineStream(ctx context.Context, state *model.ArticleState) error {
    prompt := strings.ReplaceAll(common.Agent2OutlinePrompt, "{mainTitle}", state.Title.MainTitle)
    prompt = strings.ReplaceAll(prompt, "{subTitle}", state.Title.SubTitle)

    var contentBuilder strings.Builder

    _, err := s.llm.GenerateContent(ctx, []llms.MessageContent{
        llms.TextParts(llms.ChatMessageTypeHuman, prompt),
    }, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
        text := string(chunk)
        contentBuilder.WriteString(text)
        s.sendMessage(state.TaskID, map[string]interface{}{
            "type": "AGENT2_STREAMING", "content": text,
        })
        return nil
    }))
    if err != nil {
        return err
    }

    var outline model.OutlineResult
    if err := json.Unmarshal([]byte(contentBuilder.String()), &outline); err != nil {
        return fmt.Errorf("parse outline failed: %w", err)
    }

    state.Outline = &outline
    return nil
}

func (s *ArticleAgentService) agent3GenerateContent(ctx context.Context, state *model.ArticleState) error {
    outlineJSON, _ := json.Marshal(state.Outline.Sections)
    prompt := strings.ReplaceAll(common.Agent3ContentPrompt, "{mainTitle}", state.Title.MainTitle)
    prompt = strings.ReplaceAll(prompt, "{subTitle}", state.Title.SubTitle)
    prompt = strings.ReplaceAll(prompt, "{outline}", string(outlineJSON))

    var contentBuilder strings.Builder

    _, err := s.llm.GenerateContent(ctx, []llms.MessageContent{
        llms.TextParts(llms.ChatMessageTypeHuman, prompt),
    }, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
        text := string(chunk)
        contentBuilder.WriteString(text)
        s.sendMessage(state.TaskID, map[string]interface{}{
            "type": "AGENT3_STREAMING", "content": text,
        })
        return nil
    }))
    if err != nil {
        return err
    }

    state.Content = contentBuilder.String()
    log.Printf("智能体3：正文生成成功, length=%d", len(state.Content))
    return nil
}

func (s *ArticleAgentService) agent4AnalyzeImageRequirements(ctx context.Context, state *model.ArticleState) error {
    prompt := strings.ReplaceAll(common.Agent4ImagePrompt, "{mainTitle}", state.Title.MainTitle)
    prompt = strings.ReplaceAll(prompt, "{content}", state.Content)

    content, err := llms.GenerateFromSinglePrompt(ctx, s.llm, prompt)
    if err != nil {
        return err
    }

    var requirements []model.ImageRequirement
    if err := json.Unmarshal([]byte(content), &requirements); err != nil {
        return fmt.Errorf("parse image requirements failed: %w", err)
    }

    state.ImageRequirements = requirements
    log.Printf("智能体4：配图需求分析成功, count=%d", len(requirements))
    return nil
}

func (s *ArticleAgentService) agent5GenerateImages(ctx context.Context, state *model.ArticleState) error {
    var imageResults []model.ImageResult

    for _, req := range state.ImageRequirements {
        imageURL, err := s.pexels.SearchImage(req.Keywords)
        method := "PEXELS"

        if err != nil {
            imageURL = s.pexels.GetFallbackImage(req.Position)
            method = "PICSUM"
        }

        finalURL := s.cos.UseDirectURL(imageURL)
        result := model.ImageResult{
            Position: req.Position, URL: finalURL, Method: method,
            Keywords: req.Keywords, SectionTitle: req.SectionTitle, Description: req.Type,
        }

        imageResults = append(imageResults, result)
        s.sendMessage(state.TaskID, map[string]interface{}{
            "type": "IMAGE_COMPLETE", "image": result,
        })
    }

    state.Images = imageResults
    return nil
}

func (s *ArticleAgentService) mergeImagesIntoContent(state *model.ArticleState) {
    if len(state.Images) == 0 {
        state.FullContent = state.Content
        return
    }

    var fullContent strings.Builder

    // 在正文最前面插入封面图（position=1）
    for _, img := range state.Images {
        if img.Position == 1 {
            fullContent.WriteString(fmt.Sprintf("![封面图](%s)\n\n", img.URL))
            break
        }
    }

    lines := strings.Split(state.Content, "\n")
    for _, line := range lines {
        fullContent.WriteString(line + "\n")
        if strings.HasPrefix(line, "## ") {
            sectionTitle := strings.TrimSpace(strings.TrimPrefix(line, "## "))
            s.insertImageAfterSection(&fullContent, state.Images, sectionTitle)
        }
    }

    state.FullContent = fullContent.String()
}


func (s *ArticleAgentService) insertImageAfterSection(
    fullContent *strings.Builder, images []model.ImageResult, sectionTitle string) {
    for _, image := range images {
        if image.Position > 1 && image.SectionTitle != "" &&
            strings.Contains(sectionTitle, strings.TrimSpace(image.SectionTitle)) {
            fullContent.WriteString(fmt.Sprintf("\n![%s](%s)\n", image.Description, image.URL))
            break
        }
    }
}

