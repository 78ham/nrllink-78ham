package main

// aprsPlugin 将原 APRS 模块（aprs.go）包装为内置插件：
//  1. OnLoad 后台 goroutine 运行原有重连循环，并做 panic 隔离；
//  2. OnUnload 通过 Aprs.RequestStop() 通知循环退出并释放 TCP/定时器资源；
//  3. APRS 配置仍读取 conf.APRS（保持 udphub.yaml 配置方式不变），
//     插件自身的 plugin_status.config 预留给后续覆盖项。

type aprsPlugin struct {
	core *Aprs
}

func newAprsPlugin() *aprsPlugin {
	return &aprsPlugin{core: NewAPRS()}
}

func (p *aprsPlugin) Name() string {
	return "aprs"
}

func (p *aprsPlugin) OnLoad(ctx *PluginContext) error {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ctx.Log.Printf("运行 panic: %v，插件任务已退出", r)
			}
		}()
		p.core.OnLoad()
	}()
	return nil
}

func (p *aprsPlugin) OnUnload() {
	p.core.RequestStop()
}

// 确保编译期实现 Plugin 接口
var _ Plugin = (*aprsPlugin)(nil)
