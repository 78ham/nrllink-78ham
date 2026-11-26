package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func (j *jsonapi) httpGetRoutes(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	if _, err := checktoken(w, req); err != nil {
		return
	}

	var value string
	err := db.QueryRow("SELECT COALESCE(routes, '') FROM routes ORDER BY id LIMIT 1").Scan(&value)
	if err != nil && err != sql.ErrNoRows {
		log.Println("query routes:", err)
		writeJSONResponseOpError(w)
		return
	}
	if strings.TrimSpace(value) == "" {
		value = "[]"
	}
	if !json.Valid([]byte(value)) {
		writeJSONResponseError(w, "路由配置不是有效的 JSON")
		return
	}
	writeJSONEnvelope(w, 20000, json.RawMessage(value))
}

func (j *jsonapi) httpSetRoutes(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		writeJSONResponseRightError(w)
		return
	}
	body, ok := readRequestBody(w, req)
	if !ok {
		return
	}
	if !json.Valid(body) {
		writeJSONResponseParamError(w)
		return
	}

	// 首次写入创建记录，升级已有数据库时沿用原记录的 ID。
	_, err = db.Exec(`INSERT INTO routes (id, routes)
		VALUES ((SELECT COALESCE(MIN(id), 1) FROM routes), ?)
		ON CONFLICT(id) DO UPDATE SET routes=excluded.routes`, string(body))
	if err != nil {
		log.Println("save routes:", err)
		writeJSONResponseOpError(w)
		return
	}
	addOperatorLog("", "routes修改成功", u)
	writeJSONResponseMessage(w, "routes修改成功", 20000)
}
