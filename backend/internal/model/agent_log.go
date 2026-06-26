package model

import "time"

// AgentLog 智能体执行日志
type AgentLog struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                         // 主键 ID
	TaskID       string     `gorm:"column:taskId;type:varchar(64);not null;index" json:"taskId"`          // 关联的文章任务 ID（UUID）
	AgentName    string     `gorm:"column:agentName;type:varchar(64);not null" json:"agentName"`          // 智能体名称（如 title_generator、outline_generator）
	StartTime    time.Time  `gorm:"column:startTime;not null" json:"startTime"`                             // 执行开始时间
	EndTime      *time.Time `gorm:"column:endTime" json:"endTime"`                                          // 执行结束时间（进行中为 nil）
	DurationMs   *int       `gorm:"column:durationMs" json:"durationMs"`                                    // 执行耗时（毫秒）
	Status       string     `gorm:"column:status;type:varchar(20);not null" json:"status"`                  // 执行状态：RUNNING / SUCCESS / FAILED
	ErrorMessage *string    `gorm:"column:errorMessage;type:text" json:"errorMessage"`                    // 失败时的错误信息
	Prompt       *string    `gorm:"column:prompt;type:text" json:"prompt"`                                // 发送给 LLM 的 Prompt 内容
	InputData    *string    `gorm:"column:inputData;type:text" json:"inputData"`                            // 输入数据（JSON 字符串）
	OutputData   *string    `gorm:"column:outputData;type:text" json:"outputData"`                          // 输出数据（JSON 字符串）
	CreateTime   time.Time  `gorm:"column:createTime;autoCreateTime" json:"createTime"`                   // 记录创建时间
	UpdateTime   time.Time  `gorm:"column:updateTime;autoUpdateTime" json:"updateTime"`                   // 记录更新时间
	IsDelete     int        `gorm:"column:isDelete;default:0" json:"isDelete"`                            // 软删除标记：0 正常，1 已删除
}

// TableName 指定表名
func (AgentLog) TableName() string {
	return "agent_log"
}
