package main

import (
	"fmt"
	"log"
	"net/http"
)

// httpRegisterServer 允许任意已登录用户“登记我的服务器”：
// 与 admin 的 /server/create 不同，此接口面向普通登录用户，
// 归属字段（ower_id / ower_callsign）强制取自当前 token 身份，
// 用户不能冒名登记他人服务器。status 默认 2（关闭），
// 是否启动由管理员在后台审核后开启，避免普通用户直接拉起 UDP 转发。
func (j *jsonapi) httpRegisterServer(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	// 仅允许正常状态用户登记
	if u.Status != 1 {
		w.Write([]byte(`{"code":20001,"data":{"message":"当前账号状态异常，无法登记服务器"}}`))
		return
	}

	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	stb := &Server{}
	if err := jsonextra.Unmarshal(result, stb); err != nil {
		log.Println("server register err:", err)
		w.Write([]byte(`{"code":20001,"data":{"message":"登记服务器失败,json格式错误"}}`))
		return
	}

	// 基本校验：名称与 IP/端口（或域名）必填
	if stb.Name == "" {
		w.Write([]byte(`{"code":20001,"data":{"message":"服务器名称不能为空"}}`))
		return
	}
	if stb.IPAddr == "" && stb.DNSName == "" {
		w.Write([]byte(`{"code":20001,"data":{"message":"请填写 IP 地址或域名"}}`))
		return
	}
	if stb.UDPPort == "" {
		stb.UDPPort = "60051"
	}

	// 归属强制取当前用户
	stb.ID = 0
	stb.OwerID = u.ID
	stb.OwerCallsign = u.CallSign
	// 默认关闭，等待管理员审核
	if stb.Status != 1 && stb.Status != 2 {
		stb.Status = 2
	}

	if err := addServers(stb); err != nil {
		log.Println("server register add err:", err)
		w.Write([]byte(`{"code":20001,"data":{"isok":1,"message":"登记服务器失败"}}`))
		return
	}

	addOperatorLog(fmt.Sprintf("server:%v", stb.Name), "登记服务器成功", u)
	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"服务器登记成功，等待管理员审核"}}`))
}
