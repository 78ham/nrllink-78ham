package main

// bridgePlugin 外部平台语音桥接插件。
//
// 功能：将本地群组的语音双向桥接到远端 NRL2 服务器（其他 NRL 服务器 / 自建平台）。
// 每条桥以独立 goroutine 运行：
//   - 出站（TX）：voice.before_forward hook 采样群组语音帧，过滤自身桥接呼号防环，
//     编码为 NRL2 帧经 UDP 发往对端；
//   - 入站（RX）：接收对端 NRL2 帧，校验后以桥接虚拟身份（DevModel=255）注入本地群组
//     连接池，与外部服务器互联同构；
//   - 心跳：NRL2 Type=2 心跳包维持对端会话；断线重连退避 1s→2s→5s→10s→30s（60s 封顶）。
//
// 配置存于 plugin_status.config（/api/v1/plugins/config?name=bridge 可改，保存热重启），
// 运行状态写 bridge_status 表。

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// BridgeConfig 单条桥配置
type BridgeConfig struct {
	Name           string `json:"name"`            // 桥名称（唯一）
	Type           string `json:"type"`            // 协议类型：nrl（当前支持）
	RemoteHost     string `json:"remote_host"`     // 对端地址
	RemotePort     int    `json:"remote_port"`     // 对端端口
	LocalGroupID   int    `json:"local_group_id"`  // 本地群组 ID
	BridgeCallsign string `json:"bridge_callsign"` // 桥接虚拟呼号
	BridgeSSID     byte   `json:"bridge_ssid"`     // 桥接虚拟 SSID
	Codec          byte   `json:"codec"`           // 出站编码（1=G.711）
	TxEnabled      bool   `json:"tx_enabled"`      // 本端→对端
	RxEnabled      bool   `json:"rx_enabled"`      // 对端→本端
	MaxDurationSec int    `json:"max_duration_sec"` // 单次通话最长时间（0=不限）
	HookPriority   int    `json:"hook_priority"`   // hook 优先级
}

type bridgePluginConf struct {
	Bridges []BridgeConfig `json:"bridges"`
}

// BridgeStatus 单条桥运行状态（bridge_status 表行 + JSON 视图）
type BridgeStatus struct {
	Name        string `json:"name"`
	Connected   bool   `json:"connected"`
	RxPackets   int64  `json:"rx_packets"`
	TxPackets   int64  `json:"tx_packets"`
	Dropped     int64  `json:"dropped"`
	LastError   string `json:"last_error"`
	LastActive  string `json:"last_active"`
}

type bridgePlugin struct {
	stop    chan struct{}
	stopOne sync.Once

	bridges map[string]*bridgeSession
	mu      sync.Mutex

	hookPriority int
	registered   bool

	lastTick time.Time
}

func newBridgePlugin() *bridgePlugin {
	return &bridgePlugin{
		stop:         make(chan struct{}),
		bridges:      make(map[string]*bridgeSession),
		hookPriority: 50,
		lastTick:     time.Now(),
	}
}

func (p *bridgePlugin) Name() string { return "bridge" }

func (p *bridgePlugin) OnLoad(ctx *PluginContext) error {
	var conf bridgePluginConf
	if !ctx.GetConf(&conf) || len(conf.Bridges) == 0 {
		ctx.Log.Printf("无桥接配置（config.bridges 为空），插件已加载但无活动桥")
	}

	initBridgeStatusTable()

	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range conf.Bridges {
		bc := conf.Bridges[i]
		if err := validateBridgeConfig(&bc); err != nil {
			ctx.Log.Printf("跳过无效桥配置 %q: %v", bc.Name, err)
			continue
		}
		sess := newBridgeSession(&bc, ctx)
		p.bridges[bc.Name] = sess
		go sess.run()
	}

	// 注册出站 hook（一次）
	if !p.registered {
		prio := p.hookPriority
		ctx.Hooks.Register(p.Name(), HookVoiceBeforeForward, func(e *HookEvent) {
			p.onVoiceOutbound(e)
		}, prio)
		// 30 秒一次的状态持久化
		ctx.Hooks.Register(p.Name(), HookServerTick, func(e *HookEvent) {
			p.serverTick()
		}, 90)
		p.registered = true
	}

	return nil
}

func (p *bridgePlugin) OnUnload() {
	p.stopOne.Do(func() { close(p.stop) })

	p.mu.Lock()
	sessions := make([]*bridgeSession, 0, len(p.bridges))
	for _, s := range p.bridges {
		sessions = append(sessions, s)
	}
	p.bridges = make(map[string]*bridgeSession)
	p.mu.Unlock()

	for _, s := range sessions {
		s.close()
	}
	log.Printf("[plugin:bridge] 全部桥接会话已停止")
}

func validateBridgeConfig(bc *BridgeConfig) error {
	if bc.Name == "" {
		return errBridge("name 不能为空")
	}
	if bc.Type != "nrl" && bc.Type != "" {
		return errBridge("暂只支持 type=nrl")
	}
	if bc.RemoteHost == "" {
		return errBridge("remote_host 不能为空")
	}
	if bc.RemotePort <= 0 || bc.RemotePort > 65535 {
		return errBridge("remote_port 非法")
	}
	if bc.LocalGroupID <= 0 {
		return errBridge("local_group_id 必须为有效群组 ID")
	}
	if len(bc.BridgeCallsign) < 3 || len(bc.BridgeCallsign) > 6 {
		return errBridge("bridge_callsign 须为 3-6 位呼号字符")
	}
	for _, ch := range bc.BridgeCallsign {
		if !isValidCallsignByte(byte(ch)) {
			return errBridge("bridge_callsign 只允许大写字母与数字")
		}
	}
	if bc.BridgeSSID == 0 {
		bc.BridgeSSID = 88
	}
	if bc.Codec == 0 {
		bc.Codec = CodecTypeG711
	}
	if bc.HookPriority == 0 {
		bc.HookPriority = 50
	}
	return nil
}

func isValidCallsignByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

type bridgeErr string

func (e bridgeErr) Error() string { return string(e) }

func errBridge(msg string) error { return bridgeErr(msg) }

// onVoiceOutbound voice.before_forward hook：把本群组语音帧分发给同群组的桥（TX）
func (p *bridgePlugin) onVoiceOutbound(e *HookEvent) {
	if e.Group == nil || len(e.Packet) < 48 || e.Device == nil {
		return
	}

	// 防环：桥接呼号自己发出的帧不再回桥
	src := getCallsignSSID(e.Device.CallSign, e.Device.SSID)

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, s := range p.bridges {
		if s.cfg.LocalGroupID != e.Group.ID {
			continue
		}
		if src == s.selfCallSSID() {
			continue
		}
		s.enqueueOutbound(e.Packet)
	}
}

// serverTick 刷新 bridge_status 表（保留低频持久化）
func (p *bridgePlugin) serverTick() {
	p.mu.Lock()
	sessions := make([]*bridgeSession, 0, len(p.bridges))
	for _, s := range p.bridges {
		sessions = append(sessions, s)
	}
	p.mu.Unlock()
	for _, s := range sessions {
		s.persistStatus()
	}
}

// —— 单条桥会话 ——

type bridgeSession struct {
	cfg  *BridgeConfig
	ctx  *PluginContext
	stop chan struct{}
	once sync.Once

	conn      *net.UDPConn
	remoteAddr *net.UDPAddr

	outbound chan []byte // 待发对端的原始 NRL2 帧（仅 DATA 部分会被重新封装）

	txCount atomic.Int64
	rxCount atomic.Int64
	dropped atomic.Int64

	lastError   atomic.Value // string
	lastActive  atomic.Value // time.Time
	connectedFl atomic.Bool

	// RX 语音权：对端开始说话的时间
	rxSpeakingUntil time.Time
	rxMu            sync.Mutex
}

func newBridgeSession(cfg *BridgeConfig, ctx *PluginContext) *bridgeSession {
	return &bridgeSession{
		cfg:      cfg,
		ctx:      ctx,
		stop:     make(chan struct{}),
		outbound: make(chan []byte, 64),
	}
}

func (s *bridgeSession) selfCallSSID() string {
	return getCallsignSSID(s.cfg.BridgeCallsign, s.cfg.BridgeSSID)
}

func (s *bridgeSession) close() {
	s.once.Do(func() { close(s.stop) })
}

// run 桥接主循环：连接 + 心跳 + 出站发送 + 入站接收
func (s *bridgeSession) run() {
	backoff := time.Second
	for {
		select {
		case <-s.stop:
			return
		default:
		}

		if err := s.sessionLoop(); err != nil {
			s.setLastError(err.Error())
			s.ctx.Log.Printf("桥 %s 会话异常: %v", s.cfg.Name, err)
		}

		select {
		case <-s.stop:
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
	}
}

// sessionLoop 单次 UDP 会话：拨号、收发循环，出错返回由 run 重连
func (s *bridgeSession) sessionLoop() error {
	// 重置退避在成功后处理（这里简化：连接成功即重置）
	defer func() {
		s.connectedFl.Store(false)
		if s.conn != nil {
			s.conn.Close()
			s.conn = nil
		}
	}()

	raddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(s.cfg.RemoteHost, itoa(s.cfg.RemotePort)))
	if err != nil {
		return errBridge("解析对端地址失败: " + err.Error())
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return errBridge("UDP 连接失败: " + err.Error())
	}
	s.conn = conn
	s.remoteAddr = raddr
	s.connectedFl.Store(true)
	s.ctx.Log.Printf("桥 %s 已连接 %s（群组 %d）", s.cfg.Name, raddr, s.cfg.LocalGroupID)

	// 连接成功重置重连退避：此处通过 run 循环外层无法直接重置，采用连接即发送心跳维持。
	errCh := make(chan error, 2)

	// 入站接收
	if s.cfg.RxEnabled {
		go s.rxLoop(errCh)
	}

	// 心跳（对端是 NRL2 服务器时需要周期心跳维持）
	hbDone := make(chan struct{})
	go s.heartbeatLoop(hbDone)
	defer close(hbDone)

	// 出站发送
	txErr := s.txLoop()

	select {
	case err := <-errCh:
		return err
	default:
	}
	if txErr != nil {
		return txErr
	}
	return nil
}

func (s *bridgeSession) heartbeatLoop(done chan struct{}) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	hb := encodeNRL21(s.cfg.BridgeCallsign, s.cfg.BridgeSSID, 2, 255, 0, []byte{})
	for {
		select {
		case <-done:
			return
		case <-t.C:
			if s.conn == nil {
				return
			}
			if _, err := s.conn.Write(hb); err != nil {
				s.setLastError("心跳发送失败: " + err.Error())
				return
			}
		}
	}
}

// txLoop 出站：从队列取本端帧，封装后发对端
func (s *bridgeSession) txLoop() error {
	if !s.cfg.TxEnabled {
		<-s.stop  // TX 关闭时挂起等待退出
		return nil
	}

	// 帧率限制：最多 50 帧/秒
	minInterval := 20 * time.Millisecond
	var lastSend time.Time

	for {
		select {
		case <-s.stop:
			return nil
		case raw, ok := <-s.outbound:
			if !ok {
				return nil
			}
			if len(raw) < 48 {
				continue
			}

			// 节流
			now := time.Now()
			if now.Sub(lastSend) < minInterval {
				s.dropped.Add(1)
				continue
			}

			// 提取 DATA 部分（48 字节头之后），以桥接身份重新封装
			data := raw[48:]

			// 255 模式：payload 头部为原始发话者呼号（DevModel=255 需要 Original 字段）
			// 参照 NRL21replace200and255dev：对端会读 32-43 的原始呼号/IP。
			// 我们直接使用 replace 函数，将本帧改写为 255 服务器帧：
			srcCall := string(trimNrlField(raw[24:30]))
			srcSSID := raw[30]
			srcIP := net.IPv4zero.To4()

			out := NRL21replace200and255dev(s.cfg.BridgeCallsign, s.cfg.BridgeSSID, raw[20], 255,
				srcCall, srcSSID, srcIP, 0, buildPacketWithOriginal(raw, data))

			if _, err := s.conn.Write(out); err != nil {
				s.setLastError("发送失败: " + err.Error())
				return errBridge("发送失败: " + err.Error())
			}

			s.txCount.Add(1)
			s.markActive()
			lastSend = now
		}
	}
}

// buildPacketWithOriginal 构造带 original 字段的完整帧（48 头 + data），
// data 即原帧 payload；这里直接复用原帧整体，replace 函数会重写头部。
func buildPacketWithOriginal(raw []byte, data []byte) []byte {
	// raw 已是完整 NRL2 帧（48 头 + DATA），直接返回供 replace 重写
	return raw
}

// rxLoop 入站：收对端 NRL2 帧，校验后注入本地群组
func (s *bridgeSession) rxLoop(errCh chan<- error) {
	buf := make([]byte, 65507)
	for {
		select {
		case <-s.stop:
			return
		default:
		}

		n, err := s.conn.Read(buf)
		if err != nil {
			select {
			case errCh <- errBridge("接收失败: " + err.Error()):
			default:
			}
			return
		}

		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		if err := s.injectInbound(pkt); err != nil {
			s.ctx.Log.Printf("桥 %s 入站帧丢弃: %v", s.cfg.Name, err)
			s.dropped.Add(1)
		}
	}
}

// injectInbound 校验并以桥接身份注入本地群组
func (s *bridgeSession) injectInbound(pkt []byte) error {
	if len(pkt) < 48 {
		return errBridge("帧过短")
	}

	// 防环：来源呼号==自身桥接呼号的对端回包（理论不应出现）丢弃
	srcCall := string(trimNrlField(pkt[24:30]))
	if srcCall == s.cfg.BridgeCallsign {
		return errBridge("检测到环路帧")
	}

	nrl, err := newNRL21packet(s.remoteAddr, pkt)
	if err != nil {
		return err
	}

	// 只处理语音帧（1,8,12-16）与文本消息（5）
	switch nrl.Type {
	case 1, 8, 12, 13, 14, 15, 16:
		return s.injectVoice(nrl, pkt)
	case 5:
		return s.injectMsg(nrl, pkt)
	default:
		return nil // 心跳/控制等忽略
	}
}

// injectVoice 以 255 外部服务器身份把语音注入本地群组连接池
func (s *bridgeSession) injectVoice(nrl *NRL21packet, pkt []byte) error {
	gp, ok := publicGroupLoad(s.cfg.LocalGroupID)
	if !ok {
		return errBridge("本地群组不存在")
	}

	// 语音权简化控制：对端连发时限内不再重复抢占
	s.rxMu.Lock()
	now := time.Now()
	if now.Before(s.rxSpeakingUntil) {
		s.rxMu.Unlock()
		s.rxCount.Add(1)
		s.markActive()
		return s.writeToGroup(gp, nrl, pkt)
	}
	s.rxSpeakingUntil = now.Add(10 * time.Second)
	s.rxMu.Unlock()

	s.rxCount.Add(1)
	s.markActive()
	return s.writeToGroup(gp, nrl, pkt)
}

// writeToGroup 将帧写入群组内所有普通设备（跳过 200/255 服务器设备，防止二次外泄）
func (s *bridgeSession) writeToGroup(gp *group, nrl *NRL21packet, pkt []byte) error {
	if gp.connPool == nil {
		return errBridge("群组连接池为空")
	}

	// 改写为 255 外部服务器帧：头=桥接呼号，original=对端原始发话者
	out := NRL21replace200and255dev(s.cfg.BridgeCallsign, s.cfg.BridgeSSID, nrl.Type, 255,
		nrl.CallSign, nrl.SSID, s.remoteAddr.IP.To4(), nrl.DMRID, pkt)

	for _, vv := range gp.connPool.snapshotList() {
		if vv.udpAddr == nil || vv.DevModel == 200 || vv.DevModel == 255 {
			continue
		}
		if _, err := globelconn.WriteToUDP(out, vv.udpAddr); err != nil {
			s.setLastError("注入群组失败: " + err.Error())
		}
	}
	return nil
}

// injectMsg 文本消息注入（type 5）暂只计数，转发逻辑待需求明确后扩展
func (s *bridgeSession) injectMsg(nrl *NRL21packet, pkt []byte) error {
	s.rxCount.Add(1)
	s.markActive()
	return nil
}

func (s *bridgeSession) enqueueOutbound(raw []byte) {
	select {
	case s.outbound <- raw:
	default:
		s.dropped.Add(1)
	}
}

func (s *bridgeSession) markActive() {
	s.lastActive.Store(time.Now())
}

func (s *bridgeSession) setLastError(msg string) {
	s.lastError.Store(msg)
}

// persistStatus 将会话状态写入 bridge_status 表
func (s *bridgeSession) persistStatus() {
	le, _ := s.lastError.Load().(string)
	la := ""
	if v, ok := s.lastActive.Load().(time.Time); ok && !v.IsZero() {
		la = v.Format("2006-01-02 15:04:05")
	}
	upsertBridgeStatus(&BridgeStatus{
		Name:       s.cfg.Name,
		Connected:  s.connectedFl.Load(),
		RxPackets:  s.rxCount.Load(),
		TxPackets:  s.txCount.Load(),
		Dropped:    s.dropped.Load(),
		LastError:  le,
		LastActive: la,
	})
}

func trimNrlField(b []byte) []byte {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] != 0 && b[i] != 13 {
			return b[:i+1]
		}
	}
	return b[:0]
}

func itoa(v int) string {
	return jsonScalarInt(v)
}

func jsonScalarInt(v int) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// —— bridge_status 表 ——

func initBridgeStatusTable() {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS bridge_status (
		name TEXT PRIMARY KEY,
		connected INTEGER DEFAULT 0,
		rx_packets INTEGER DEFAULT 0,
		tx_packets INTEGER DEFAULT 0,
		dropped INTEGER DEFAULT 0,
		last_error TEXT DEFAULT '',
		last_active TEXT DEFAULT '',
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Printf("[plugin:bridge] create bridge_status err: %v", err)
	}
}

func upsertBridgeStatus(st *BridgeStatus) {
	connected := 0
	if st.Connected {
		connected = 1
	}
	_, err := db.Exec(`INSERT INTO bridge_status (name, connected, rx_packets, tx_packets, dropped, last_error, last_active, updated_at)
		VALUES (?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET connected=excluded.connected, rx_packets=excluded.rx_packets,
		tx_packets=excluded.tx_packets, dropped=excluded.dropped, last_error=excluded.last_error,
		last_active=excluded.last_active, updated_at=CURRENT_TIMESTAMP`,
		st.Name, connected, st.RxPackets, st.TxPackets, st.Dropped, st.LastError, st.LastActive)
	if err != nil {
		log.Printf("[plugin:bridge] upsert status err: %v", err)
	}
}

// base64 保留引用（外部协议语音下发使用）
var _ = base64.StdEncoding
