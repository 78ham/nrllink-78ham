package main

// 插件系统框架核心：Hook 注册表与事件派发。
// 设计原则：
//  1. 语音/信令转发是热路径，Hook 派发默认异步（goroutine），无订阅者时零开销直接返回；
//  2. 有订阅者时对 Packet 做拷贝，避免 UDP 读缓冲复用导致的数据竞争；
//  3. 插件回调 panic 一律 recover，只记日志，绝不拖垮主服务。

import (
	"log"
	"sort"
	"sync"
	"time"
)

// —— Hook 名称常量 ——
const (
	HookVoiceBeforeForward = "voice.before_forward" // 语音帧转发前
	HookMsgBeforeForward   = "msg.before_forward"   // 文本消息转发前
	HookCtlBeforeForward   = "ctl.before_forward"   // 控制信令转发前
	HookDeviceOnline       = "device.online"        // 设备上线
	HookDeviceOffline      = "device.offline"       // 设备离线
	HookGroupJoin          = "group.join"           // 设备加入群组
	HookGroupLeave         = "group.leave"          // 设备离开群组
	HookServerTick         = "server.tick"          // 服务定时器（30 秒一次）
)

// HookEvent 传给插件钩子的事件
type HookEvent struct {
	Hook   string                 // 触发的 hook 名称
	Time   time.Time              // 事件时间
	Device *deviceInfo            // 相关设备（可能为 nil）
	Group  *group                 // 相关群组（可能为 nil）
	Packet []byte                 // 原始 NRL2 包（可能为 nil，异步派发时为拷贝副本）
	Data   map[string]interface{} // 扩展字段
}

// HookFunc 插件钩子回调
type HookFunc func(e *HookEvent)

type hookEntry struct {
	plugin   string // 注册该回调的插件名
	priority int    // 数值越小越先执行
	fn       HookFunc
}

// HookRegistry Hook 注册表，全局唯一实例 hookRegistry
type HookRegistry struct {
	mu      sync.RWMutex
	entries map[string][]hookEntry
}

func NewHookRegistry() *HookRegistry {
	return &HookRegistry{entries: make(map[string][]hookEntry)}
}

// Register 注册 hook 回调；同一 hook 内按 priority 升序执行
func (h *HookRegistry) Register(pluginName, hook string, fn HookFunc, priority int) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries[hook] = append(h.entries[hook], hookEntry{plugin: pluginName, priority: priority, fn: fn})
}

// UnregisterAll 注销某插件注册的全部回调（插件卸载时调用）
func (h *HookRegistry) UnregisterAll(pluginName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for hook, list := range h.entries {
		kept := list[:0]
		for _, en := range list {
			if en.plugin != pluginName {
				kept = append(kept, en)
			}
		}
		if len(kept) == 0 {
			delete(h.entries, hook)
		} else {
			h.entries[hook] = kept
		}
	}
}

// lookup 返回按 priority 排序的订阅者快照
func (h *HookRegistry) lookup(hook string) []hookEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := h.entries[hook]
	if len(list) == 0 {
		return nil
	}
	sorted := make([]hookEntry, len(list))
	copy(sorted, list)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].priority < sorted[j].priority })
	return sorted
}

// Dispatch 同步派发：按 priority 顺序在当前 goroutine 执行，
// 仅供明确知晓开销的调用方使用（普通业务请用 DispatchAsync）
func (h *HookRegistry) Dispatch(e *HookEvent) {
	for _, en := range h.lookup(e.Hook) {
		h.safeCall(en, e)
	}
}

// DispatchAsync 异步派发：无订阅者直接返回（零开销）；
// 有订阅者时拷贝 Packet 后每个订阅者独立 goroutine 执行
func (h *HookRegistry) DispatchAsync(e *HookEvent) {
	entries := h.lookup(e.Hook)
	if len(entries) == 0 {
		return
	}

	// 异步派发时 packet 指向 UDP 读缓冲，会被复用，必须拷贝
	if len(e.Packet) > 0 {
		buf := make([]byte, len(e.Packet))
		copy(buf, e.Packet)
		e.Packet = buf
	}

	for _, en := range entries {
		en := en
		go func(ev *HookEvent) {
			h.safeCall(en, ev)
		}(e)
	}
}

// safeCall 单个回调的 panic 隔离
func (h *HookRegistry) safeCall(en hookEntry, e *HookEvent) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[plugin] hook %s 回调 panic (plugin=%s): %v", e.Hook, en.plugin, r)
		}
	}()
	en.fn(e)
}
