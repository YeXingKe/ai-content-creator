package service

// CheckAndConsumeQuota 检查并消耗配额（原子操作）
func (s *QuotaService) CheckAndConsumeQuota(user *model.User) error {
    // 管理员跳过检查
    if s.isAdmin(user) {
        return nil
    }

    // 原子更新：检查与消费合并为一个 SQL
    affectedRows, err := s.userStore.DecrementQuota(user.ID)
    if err != nil {
        return common.ErrSystem
    }

    if affectedRows == 0 {
        // 影响行数为0，说明配额不足
        return common.ErrOperation.WithMessage("配额不足，无法创建文章")
    }

    log.Printf("用户配额检查并消耗成功, userId=%d", user.ID)
    return nil
}
