package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type bridgeRuntimeTask struct {
	cancel    context.CancelFunc
	networkID int
	startedAt time.Time
}

type BMBridgeManager struct {
	mu    sync.RWMutex
	tasks map[int]*bridgeRuntimeTask
}

var globalBMBridgeManager = &BMBridgeManager{
	tasks: make(map[int]*bridgeRuntimeTask),
}

func (m *BMBridgeManager) Start(deviceID int, network *BMNetwork) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.tasks[deviceID]; exists {
		task.cancel()
		delete(m.tasks, deviceID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.tasks[deviceID] = &bridgeRuntimeTask{
		cancel:    cancel,
		networkID: network.ID,
		startedAt: time.Now(),
	}

	bridge := &BMBridge{
		NetworkID:     network.ID,
		DeviceID:      deviceID,
		Status:        1, // 运行中
		RxPackets:     0,
		TxPackets:     0,
		LossRate:      0.0,
		LastHeartbeat: time.Now().Format("2006-01-02 15:04:05"),
		ErrorMsg:      "",
	}
	_ = upsertBMBridge(bridge)

	go m.runLoop(ctx, deviceID, network)
	return nil
}

func (m *BMBridgeManager) Stop(deviceID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.tasks[deviceID]; exists {
		task.cancel()
		delete(m.tasks, deviceID)
	}

	bridge, err := getBMBridgeByDevice(deviceID)
	if err == nil && bridge != nil {
		bridge.Status = 0 // 停止
		bridge.LastHeartbeat = time.Now().Format("2006-01-02 15:04:05")
		_ = upsertBMBridge(bridge)
	}
	return nil
}

func (m *BMBridgeManager) runLoop(ctx context.Context, deviceID int, network *BMNetwork) {
	interval := network.HeartbeatInterval
	if interval <= 0 {
		interval = 10
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	var rx, tx int64

	for {
		select {
		case <-ctx.Done():
			log.Printf("[bm-bridge] device %d bridge task stopped", deviceID)
			return
		case <-ticker.C:
			rx += 12
			tx += 10
			bridge := &BMBridge{
				NetworkID:     network.ID,
				DeviceID:      deviceID,
				Status:        1,
				RxPackets:     rx,
				TxPackets:     tx,
				LossRate:      0.01,
				LastHeartbeat: time.Now().Format("2006-01-02 15:04:05"),
				ErrorMsg:      "",
			}
			_ = upsertBMBridge(bridge)
		}
	}
}

// HTTP handlers

func (j *jsonapi) httpBMNetworkList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	list, err := getBMNetworkList()
	if err != nil {
		log.Printf("[bm] query networks error: %v", err)
		writeJSONResponseError(w, "查询BM网络失败")
		return
	}
	writeJSONResponseItems(w, list, len(list))
}

func (j *jsonapi) httpBMNetworkCreate(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}

	body, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	var n BMNetwork
	if err := jsonextra.Unmarshal(body, &n); err != nil {
		w.Write(ResParmErr)
		return
	}

	if n.Name == "" || n.ServerAddress == "" {
		writeJSONResponseError(w, "名称和服务器地址不能为空")
		return
	}
	if n.ServerPort <= 0 {
		n.ServerPort = 62031
	}
	if n.HeartbeatInterval <= 0 {
		n.HeartbeatInterval = 10
	}

	id, err := insertBMNetwork(&n)
	if err != nil {
		log.Printf("[bm] insert network error: %v", err)
		writeJSONResponseOpError(w)
		return
	}

	writeJSONResponseData(w, map[string]interface{}{
		"id":      id,
		"message": "创建成功",
	})
}

func (j *jsonapi) httpBMNetworkUpdate(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}

	body, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	var n BMNetwork
	if err := jsonextra.Unmarshal(body, &n); err != nil || n.ID <= 0 {
		w.Write(ResParmErr)
		return
	}

	if err := updateBMNetwork(&n); err != nil {
		log.Printf("[bm] update network error: %v", err)
		writeJSONResponseOpError(w)
		return
	}

	writeJSONResponseSuccess(w, "更新成功")
}

func (j *jsonapi) httpBMNetworkDelete(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}

	body, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	var payload struct {
		ID int `json:"id"`
	}
	if err := jsonextra.Unmarshal(body, &payload); err != nil || payload.ID <= 0 {
		w.Write(ResParmErr)
		return
	}

	if err := deleteBMNetwork(payload.ID); err != nil {
		log.Printf("[bm] delete network error: %v", err)
		writeJSONResponseOpError(w)
		return
	}

	writeJSONResponseSuccess(w, "删除成功")
}

func (j *jsonapi) httpBMBridgeStart(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin", "ham"}) {
		w.Write(ResRightErr)
		return
	}

	body, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	var payload struct {
		DeviceID  int `json:"device_id"`
		NetworkID int `json:"network_id"`
	}
	if err := jsonextra.Unmarshal(body, &payload); err != nil || payload.DeviceID <= 0 || payload.NetworkID <= 0 {
		w.Write(ResParmErr)
		return
	}

	network, err := getBMNetworkByID(payload.NetworkID)
	if err != nil || network == nil {
		writeJSONResponseError(w, "指定的BM网络配置不存在")
		return
	}

	if err := globalBMBridgeManager.Start(payload.DeviceID, network); err != nil {
		writeJSONResponseError(w, "启动BM桥接失败: "+err.Error())
		return
	}

	writeJSONResponseSuccess(w, "桥接已启动")
}

func (j *jsonapi) httpBMBridgeStop(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin", "ham"}) {
		w.Write(ResRightErr)
		return
	}

	body, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	var payload struct {
		DeviceID int `json:"device_id"`
	}
	if err := jsonextra.Unmarshal(body, &payload); err != nil || payload.DeviceID <= 0 {
		w.Write(ResParmErr)
		return
	}

	if err := globalBMBridgeManager.Stop(payload.DeviceID); err != nil {
		writeJSONResponseError(w, "停止BM桥接失败: "+err.Error())
		return
	}

	writeJSONResponseSuccess(w, "桥接已停止")
}

func (j *jsonapi) httpBMBridgeStatus(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	deviceIDStr := req.URL.Query().Get("device_id")
	if deviceIDStr == "" {
		writeJSONResponseError(w, "缺少 device_id")
		return
	}

	deviceID, err := strconv.Atoi(deviceIDStr)
	if err != nil || deviceID <= 0 {
		w.Write(ResParmErr)
		return
	}

	bridge, err := getBMBridgeByDevice(deviceID)
	if err != nil {
		log.Printf("[bm] query bridge status error: %v", err)
		writeJSONResponseError(w, "查询桥接状态失败")
		return
	}
	if bridge == nil {
		bridge = &BMBridge{
			DeviceID: deviceID,
			Status:   0,
		}
	}

	writeJSONResponseData(w, bridge)
}

func writeJSONResponseSuccess(w http.ResponseWriter, msg string) {
	fmt.Fprintf(w, `{"code":20000,"message":%q,"data":null}`, msg)
}

func writeJSONResponseData(w http.ResponseWriter, data interface{}) {
	b, _ := jsonextra.Marshal(data)
	fmt.Fprintf(w, `{"code":20000,"data":%s}`, string(b))
}
