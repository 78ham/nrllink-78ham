package main

// 插件系统管理器：插件注册、生命周期、启用状态与配置持久化、管理 HTTP API。
// 插件状态存于 plugin_status 表，管理员可通过 /api/v1/plugins* 接口启停插件。

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Plugin 插件接口：所有内置插件实现该接口
type Plugin interface {
	Name() string                      // 唯一标识，如 "aprs"
	OnLoad(ctx *PluginContext) error   // 加载：注册 hook / 启动后台任务（阻塞任务请放 goroutine）
	OnUnload()                         // 卸载：释放资源、停止任务
}

// PluginContext 传给插件的运行上下文
type PluginContext struct {
	Name  string           // 插件名
	Conf  json.RawMessage  // 插件自身配置（plugin_status.config 中的 JSON）
	Hooks *HookRegistry    // 全局 Hook 注册表
	Log   *log.Logger      // 带 [plugin:<name>] 前缀的日志器
}

// GetConf 将插件配置解码到 dst；config 为空或解析失败时返回 false
func (c *PluginContext) GetConf(dst interface{}) bool {
	if len(c.Conf) == 0 {
		return false
	}
	return json.Unmarshal(c.Conf, dst) == nil
}

// SetConf 将插件配置编码为 JSON（供保存配置接口使用）
func (c *PluginContext) SetConf(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type pluginMeta struct {
	description string
	factory     func() Plugin
}

// PluginStatus 插件运行状态快照（供管理接口展示）
type PluginStatus struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Running     bool   `json:"running"`
}

var (
	// pluginRegistry 内置插件注册表：name -> 元信息
	pluginRegistry = map[string]pluginMeta{}
	// hookRegistry 全局 Hook 注册表
	hookRegistry = NewHookRegistry()

	pluginMu       sync.Mutex
	runningPlugins = map[string]*runningPlugin{}
)

type runningPlugin struct {
	plugin Plugin
	ctx    *PluginContext
}

// registerPlugin 注册内置插件（在 pluginLoadAll 或包初始化时调用）
func registerPlugin(name, description string, factory func() Plugin) {
	pluginRegistry[name] = pluginMeta{description: description, factory: factory}
}

// initPluginStatusTable 建插件状态表
func initPluginStatusTable() {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS plugin_status (
		name TEXT PRIMARY KEY,
		enabled INTEGER DEFAULT 1,
		config TEXT DEFAULT '{}',
		updated_at TEXT
	);`)
	if err != nil {
		log.Printf("[plugin] create plugin_status table err: %v", err)
	}
}

// pluginGetStatus 读取插件启用状态与配置；无记录时默认启用、空配置
func pluginGetStatus(name string) (enabled bool, confStr string) {
	var enabledInt int
	err := db.QueryRow(`SELECT enabled, COALESCE(config, '{}') FROM plugin_status WHERE name=?`, name).
		Scan(&enabledInt, &confStr)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[plugin] query status %s err: %v", name, err)
		}
		return true, "{}"
	}
	return enabledInt != 0, confStr
}

// pluginSetStatus 写入启用状态/配置
func pluginSetStatus(name string, enabled *bool, config *string) error {
	// 保证有基础行
	if _, err := db.Exec(`INSERT OR IGNORE INTO plugin_status(name, enabled, config, updated_at)
		VALUES(?, 1, '{}', CURRENT_TIMESTAMP)`, name); err != nil {
		return err
	}
	if enabled != nil {
		v := 0
		if *enabled {
			v = 1
		}
		if _, err := db.Exec(`UPDATE plugin_status SET enabled=?, updated_at=CURRENT_TIMESTAMP WHERE name=?`, v, name); err != nil {
			return err
		}
	}
	if config != nil {
		if _, err := db.Exec(`UPDATE plugin_status SET config=?, updated_at=CURRENT_TIMESTAMP WHERE name=?`, *config, name); err != nil {
			return err
		}
	}
	return nil
}

// pluginLoadAll 插件系统入口：建表、注册内置插件、加载已启用插件、启动定时器
func pluginLoadAll() {
	initPluginStatusTable()

	// —— 内置插件注册表（新插件在此追加）——
	registerPlugin("aprs", "APRS-IS 服务器定位上报与在线统计广播", func() Plugin { return newAprsPlugin() })
	registerPlugin("bridge", "外部平台语音桥接（其他 NRL 服务器/自建平台桥入）", func() Plugin { return newBridgePlugin() })

	names := make([]string, 0, len(pluginRegistry))
	for name := range pluginRegistry {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		enabled, _ := pluginGetStatus(name)
		if !enabled {
			log.Printf("[plugin] %s 已禁用，跳过加载", name)
			continue
		}
		pluginStart(name)
	}

	go pluginTickLoop()

	log.Printf("[plugin] 插件系统已启动，内置插件 %d 个", len(names))
}

// pluginStart 启动单个插件（已运行则跳过）
func pluginStart(name string) {
	pluginMu.Lock()
	defer pluginMu.Unlock()

	if _, ok := runningPlugins[name]; ok {
		return
	}
	meta, ok := pluginRegistry[name]
	if !ok {
		log.Printf("[plugin] 未知插件: %s", name)
		return
	}

	_, confStr := pluginGetStatus(name)

	inst := &runningPlugin{
		plugin: meta.factory(),
		ctx: &PluginContext{
			Name:  name,
			Conf:  json.RawMessage(confStr),
			Hooks: hookRegistry,
			Log:   log.New(log.Writer(), "[plugin:"+name+"] ", log.LstdFlags),
		},
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[plugin] %s OnLoad panic 已摘除: %v", name, r)
		}
	}()

	if err := inst.plugin.OnLoad(inst.ctx); err != nil {
		log.Printf("[plugin] %s 启动失败: %v", name, err)
		return
	}

	runningPlugins[name] = inst
	log.Printf("[plugin] %s 已加载", name)
}

// pluginStop 停止单个插件：注销 hook 并调用 OnUnload
func pluginStop(name string) {
	pluginMu.Lock()
	inst, ok := runningPlugins[name]
	if ok {
		delete(runningPlugins, name)
	}
	pluginMu.Unlock()

	if !ok {
		return
	}

	hookRegistry.UnregisterAll(name)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[plugin] %s OnUnload panic: %v", name, r)
		}
	}()

	inst.plugin.OnUnload()
	log.Printf("[plugin] %s 已停止", name)
}

// pluginEnable / pluginDisable 管理操作：改库 + 启停实例
func pluginEnable(name string) error {
	if _, ok := pluginRegistry[name]; !ok {
		return errPluginUnknown(name)
	}
	if err := pluginSetStatus(name, boolPtr(true), nil); err != nil {
		return err
	}
	pluginStart(name)
	return nil
}

func pluginDisable(name string) error {
	if _, ok := pluginRegistry[name]; !ok {
		return errPluginUnknown(name)
	}
	if err := pluginSetStatus(name, boolPtr(false), nil); err != nil {
		return err
	}
	pluginStop(name)
	return nil
}

func errPluginUnknown(name string) error {
	return &pluginErr{"未知插件: " + name}
}

type pluginErr struct{ msg string }

func (e *pluginErr) Error() string { return e.msg }

// pluginTickLoop 全局定时器：每 30 秒派发 HookServerTick
func pluginTickLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		hookRegistry.DispatchAsync(&HookEvent{Hook: HookServerTick, Time: time.Now()})
	}
}

func boolPtr(v bool) *bool { return &v }

// —— HTTP 管理 API（jsonapi 方法，路由在 http.go msghttp 中注册）——

// httpPluginList 插件列表
func (j *jsonapi) httpPluginList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		writeJSONResponseRightError(w)
		return
	}

	pluginMu.Lock()
	running := make(map[string]bool, len(runningPlugins))
	for name := range runningPlugins {
		running[name] = true
	}
	pluginMu.Unlock()

	items := make([]PluginStatus, 0, len(pluginRegistry))
	for name, meta := range pluginRegistry {
		enabled, _ := pluginGetStatus(name)
		items = append(items, PluginStatus{
			Name:        name,
			Description: meta.description,
			Enabled:     enabled,
			Running:     running[name],
		})
	}
	sort.Slice(items, func(i, k int) bool { return items[i].Name < items[k].Name })

	writeJSONResponseItems(w, items, len(items))
}

// httpPluginEnable / httpPluginDisable 请求体: {"name":"aprs"}
func (j *jsonapi) httpPluginEnable(w http.ResponseWriter, req *http.Request) {
	j.pluginToggle(w, req, true)
}

func (j *jsonapi) httpPluginDisable(w http.ResponseWriter, req *http.Request) {
	j.pluginToggle(w, req, false)
}

func (j *jsonapi) pluginToggle(w http.ResponseWriter, req *http.Request, enable bool) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		writeJSONResponseRightError(w)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if !decodeRequestJSON(w, req, &body) {
		return
	}
	if body.Name == "" {
		writeJSONResponseParamError(w)
		return
	}

	var opErr error
	if enable {
		opErr = pluginEnable(body.Name)
	} else {
		opErr = pluginDisable(body.Name)
	}
	if opErr != nil {
		log.Printf("[plugin] toggle %s enable=%v err: %v", body.Name, enable, opErr)
		writeJSONResponseError(w, opErr.Error())
		return
	}

	writeJSONResponseOK(w)
}

// httpPluginConfig GET ?name=xxx 读取插件配置 JSON
func (j *jsonapi) httpPluginConfig(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		writeJSONResponseRightError(w)
		return
	}

	name := req.URL.Query().Get("name")
	if name == "" {
		writeJSONResponseParamError(w)
		return
	}
	if _, ok := pluginRegistry[name]; !ok {
		writeJSONResponseError(w, errPluginUnknown(name).Error())
		return
	}

	_, confStr := pluginGetStatus(name)
	writeJSONEnvelope(w, 20000, struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}{Name: name, Config: json.RawMessage(confStr)})
}

// httpPluginBridgeStatus 桥接插件状态查询：GET /api/v1/plugins/bridge/status
// 返回 bridge_status 表全部行 + 插件运行状态
func (j *jsonapi) httpPluginBridgeStatus(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		writeJSONResponseRightError(w)
		return
	}

	rows, err := db.Query(`SELECT name, connected, rx_packets, tx_packets, dropped,
		COALESCE(last_error, ''), COALESCE(last_active, '') FROM bridge_status`)
	if err != nil {
		writeJSONResponseOpError(w)
		return
	}
	defer rows.Close()

	items := make([]BridgeStatus, 0, 8)
	for rows.Next() {
		var st BridgeStatus
		var connected int
		if err := rows.Scan(&st.Name, &connected, &st.RxPackets, &st.TxPackets, &st.Dropped, &st.LastError, &st.LastActive); err == nil {
			st.Connected = connected != 0
			items = append(items, st)
		}
	}

	pluginMu.Lock()
	_, running := runningPlugins["bridge"]
	pluginMu.Unlock()

	writeJSONEnvelope(w, 20000, struct {
		Running bool           `json:"running"`
		Items   []BridgeStatus `json:"items"`
	}{Running: running, Items: items})
}

// httpPluginConfigUpdate POST {"name":"aprs","config":{...}} 保存配置；插件运行中则重启生效
func (j *jsonapi) httpPluginConfigUpdate(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		writeJSONResponseRightError(w)
		return
	}

	var body struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if !decodeRequestJSON(w, req, &body) {
		return
	}
	if body.Name == "" {
		writeJSONResponseParamError(w)
		return
	}
	if _, ok := pluginRegistry[body.Name]; !ok {
		writeJSONResponseError(w, errPluginUnknown(body.Name).Error())
		return
	}
	// 校验必须是合法 JSON 对象
	if len(body.Config) == 0 || !json.Valid(body.Config) {
		writeJSONResponseParamError(w)
		return
	}

	confStr := string(body.Config)
	if err := pluginSetStatus(body.Name, nil, &confStr); err != nil {
		log.Printf("[plugin] save config %s err: %v", body.Name, err)
		writeJSONResponseOpError(w)
		return
	}

	// 运行中的插件重启以加载新配置
	pluginMu.Lock()
	wasRunning := false
	if _, ok := runningPlugins[body.Name]; ok {
		wasRunning = true
	}
	pluginMu.Unlock()
	if wasRunning {
		pluginStop(body.Name)
		pluginStart(body.Name)
	}

	writeJSONResponseOK(w)
}
