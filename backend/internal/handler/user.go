package handler

import (
	"net/http"

	"github.com/ai-content-creator/backend/internal/common"
	"github.com/ai-content-creator/backend/internal/model"
	"github.com/ai-content-creator/backend/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// 作用：HTTP 接口层（Handler / Controller） 文件：负责接收前端请求、调用 UserService、返回统一 JSON

// UserHandler 用户处理器
type UserHandler struct {
   svc *service.UserService
}
  
// NewUserHandler 创建用户处理器
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Register 用户注册
// @Summary  用户注册
// @Tags     用户
// @Accept   json
// @Produce  json
// @Param    body  body      model.RegisterRequest  true  "注册参数"
// @Success  200   {object}  common.BaseResponse{data=int64}
// @Router   /user/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	userID, err := h.svc.Register(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(userID))
}

// Login 用户登录
// @Summary  用户登录
// @Tags     用户
// @Accept   json
// @Produce  json
// @Param    body  body      model.LoginRequest  true  "登录参数"
// @Success  200   {object}  common.BaseResponse{data=model.LoginUser}
// @Router   /user/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	session := sessions.Default(c) // 当前请求的 Session
	loginUser, err := h.svc.Login(&req, session) // 调用 UserService 的 Login 方法
	if err != nil {
		handleError(c, err) // 统一错误处理
		return
	}

	c.JSON(http.StatusOK, common.Success(loginUser))
}

// GetLoginUser 获取当前登录用户
// @Summary  获取当前登录用户
// @Tags     用户
// @Produce  json
// @Success  200  {object}  common.BaseResponse{data=model.LoginUser}
// @Router   /user/get/login [get]
func (h *UserHandler) GetLoginUser(c *gin.Context) {
	session := sessions.Default(c)
	user, err := h.svc.GetLoginUser(session)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(user.ToLoginUser()))
}

// Logout 用户注销
// @Summary  用户注销
// @Tags     用户
// @Produce  json
// @Success  200  {object}  common.BaseResponse{data=bool}
// @Router   /user/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	if err := h.svc.Logout(session); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(true))
}

// Add 创建用户（管理员）
// @Summary  创建用户
// @Tags     用户管理
// @Accept   json
// @Produce  json
// @Param    body  body      model.AddUserRequest  true  "创建参数"
// @Success  200   {object}  common.BaseResponse{data=int64}
// @Router   /user/add [post]
func (h *UserHandler) Add(c *gin.Context) {
	var req model.AddUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	userID, err := h.svc.Create(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(userID))
}

// Get 根据 ID 获取用户（管理员）
// @Summary  根据 ID 获取用户
// @Tags     用户管理
// @Produce  json
// @Param    id  query     int64  true  "用户 ID"
// @Success  200  {object}  common.BaseResponse{data=model.User}
// @Router   /user/get [get]
func (h *UserHandler) Get(c *gin.Context) {
	var req struct {
		ID int64 `form:"id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	user, err := h.svc.GetByID(req.ID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(user))
}

// GetVO 根据 ID 获取用户信息
// @Summary  根据 ID 获取用户信息
// @Tags     用户
// @Produce  json
// @Param    id  query     int64  true  "用户 ID"
// @Success  200  {object}  common.BaseResponse{data=model.UserInfo}
// @Router   /user/get/vo [get]
func (h *UserHandler) GetVO(c *gin.Context) {
	var req struct {
		ID int64 `form:"id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	user, err := h.svc.GetByID(req.ID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(user.ToUserInfo()))
}

// Delete 删除用户（管理员）
// @Summary  删除用户
// @Tags     用户管理
// @Accept   json
// @Produce  json
// @Param    body  body      model.DeleteRequest  true  "删除参数"
// @Success  200   {object}  common.BaseResponse{data=bool}
// @Router   /user/delete [post]
func (h *UserHandler) Delete(c *gin.Context) {
	var req model.DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	if err := h.svc.Delete(req.ID); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(true))
}

// Update 更新用户（管理员）
// @Summary  更新用户
// @Tags     用户管理
// @Accept   json
// @Produce  json
// @Param    body  body      model.UpdateUserRequest  true  "更新参数"
// @Success  200   {object}  common.BaseResponse{data=bool}
// @Router   /user/update [post]
func (h *UserHandler) Update(c *gin.Context) {
	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	if err := h.svc.Update(&req); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(true))
}

// ListPageVO 分页查询用户列表（管理员）
// @Summary  分页查询用户列表
// @Tags     用户管理
// @Accept   json
// @Produce  json
// @Param    body  body      model.QueryUserRequest  true  "查询参数"
// @Success  200   {object}  common.BaseResponse{data=model.PageResult}
// @Router   /user/list/page/vo [post]
func (h *UserHandler) ListPageVO(c *gin.Context) {
	var req model.QueryUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, common.Error(common.ErrParams))
		return
	}

	// 设置默认值
	if req.PageNum <= 0 {
		req.PageNum = common.DefaultPageNum
	}
	if req.PageSize <= 0 {
		req.PageSize = common.DefaultPageSize
	}
	if req.PageSize > common.MaxPageSize {
		req.PageSize = common.MaxPageSize
	}

	page, err := h.svc.ListByPage(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, common.Success(page))
}

// handleError 统一错误处理
func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*common.AppError); ok {
		c.JSON(http.StatusOK, common.Error(appErr))
	} else {
		c.JSON(http.StatusOK, common.Error(common.ErrSystem))
	}
}
