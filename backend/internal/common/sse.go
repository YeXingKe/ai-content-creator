package common

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// SSEManager SSE 连接管理器
type SSEManager struct {
	clients map[string]chan string   // 任务 ID 到 SSE 通道的映射
	mu      sync.RWMutex   // 读写锁。多个 goroutine 同时注册/发送/注销时保证 map 安全
}

// NewSSEManager 创建 SSE 管理器
func NewSSEManager() *SSEManager {
	return &SSEManager{
		clients: make(map[string]chan string), // 初始化 clients 为空 map
	}
}

// Register 注册 SSE 客户端
func (m *SSEManager) Register(taskID string) chan string {
	m.mu.Lock()  // 加写锁，改 map 期间其他写操作等待；函数返回前 defer 解锁。
	defer m.mu.Unlock() // 函数返回前解锁

	ch := make(chan string, 100) // 缓冲通道，避免阻塞
	m.clients[taskID] = ch // 将任务 ID 和 SSE 通道关联起来
	return ch
}

// Unregister 注销 SSE 客户端
func (m *SSEManager) Unregister(taskID string) {
	m.mu.Lock()  // 加写锁，改 map 期间其他写操作等待；函数返回前 defer 解锁。
	defer m.mu.Unlock() // 函数返回前解锁

	if ch, ok := m.clients[taskID]; ok { // 如果任务 ID 对应的 SSE 通道存在
		close(ch) // 关闭通道，通知所有等待的读取者。
		delete(m.clients, taskID) // 删除任务 ID 对应的 SSE 通道
	}
}

// Send 发送消息
func (m *SSEManager) Send(taskID string, data interface{}) {
	m.mu.RLock()  // 加读锁，改 map 期间其他读操作等待；函数返回前 defer 解锁。
	ch, ok := m.clients[taskID] // 获取任务 ID 对应的 SSE 通道
	m.mu.RUnlock()

	if !ok {
		return
	}

	// 将数据转为 JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	// 非阻塞发送
	select {
	case ch <- string(jsonData): // 将数据转为 JSON 后发送给 SSE 通道
	case <-time.After(5 * time.Second):// 缓冲满且 5 秒内写不进去：放弃本条
		// 这是背压保护，避免业务 goroutine 永久阻塞
		// 超时则放弃
	}
}

// Complete 完成连接
func (m *SSEManager) Complete(taskID string) {
	m.Unregister(taskID)
}

// Exists 检查连接是否存在
func (m *SSEManager) Exists(taskID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[taskID]
	return ok
}

// WriteSSE 写入 SSE 消息
func WriteSSE(w io.Writer, data string) error {
	_, err := fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}