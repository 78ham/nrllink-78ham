package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

var wsConnPoll = make(map[string]*websocket.Conn, 20)
var wsConnPollMu sync.RWMutex

func upper(ws *websocket.Conn) {
	addr := ws.RemoteAddr().String()

	wsConnPollMu.Lock()
	wsConnPoll[addr] = ws
	wsConnPollMu.Unlock()

	defer func() {
		wsConnPollMu.Lock()
		delete(wsConnPoll, addr)
		wsConnPollMu.Unlock()
	}()

	var err error
	for {
		var reply string

		if err = websocket.Message.Receive(ws, &reply); err != nil {
			fmt.Println(time.Now(), err)
			continue
		}

		if err = websocket.Message.Send(ws, strings.ToUpper(reply)); err != nil {
			fmt.Println(time.Now(), err)
			continue
		}
	}
}
