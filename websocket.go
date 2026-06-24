package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

var wsConnPoll = make(map[string]*websocket.Conn, 20)
var wsConnPollMu sync.Mutex

func upper(ws *websocket.Conn) {

	remoteAddr := ws.RemoteAddr().String()
	wsConnPollMu.Lock()
	wsConnPoll[remoteAddr] = ws
	wsConnPollMu.Unlock()
	defer func() {
		wsConnPollMu.Lock()
		delete(wsConnPoll, remoteAddr)
		wsConnPollMu.Unlock()
		ws.Close()
	}()

	var err error
	for {
		var reply string

		if err = websocket.Message.Receive(ws, &reply); err != nil {
			fmt.Println(time.Now(), err)
			return
		}

		if err = websocket.Message.Send(ws, strings.ToUpper(reply)); err != nil {
			fmt.Println(time.Now(), err)
			return
		}
	}
}
