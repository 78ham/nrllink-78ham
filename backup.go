package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// httpAdminBackupDB 生成当前 SQLite/SQLCipher 数据库的一致性快照并以流式附件下发
func (j *jsonapi) httpAdminBackupDB(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}

	// 临时备份文件放置在数据库文件所在目录，避免使用操作系统默认 C 盘临时目录
	dbDir := filepath.Dir(conf.System.DBfile)
	timestamp := time.Now().Format("20060102_150405")
	tempBackupFile := filepath.Join(dbDir, fmt.Sprintf(".backup_%d_%s.sqlite3", time.Now().UnixNano(), timestamp))

	// 使用 SQLite VACUUM INTO 创建一致性快照（支持在线备份与加密数据库）
	vacuumSQL := fmt.Sprintf("VACUUM INTO '%s';", filepath.ToSlash(tempBackupFile))
	if _, err := db.Exec(vacuumSQL); err != nil {
		log.Printf("VACUUM INTO backup failed: %v", err)
		writeJSONResponseMessage(w, "生成数据库快照失败: "+err.Error(), 20001)
		return
	}
	defer os.Remove(tempBackupFile)

	f, err := os.Open(tempBackupFile)
	if err != nil {
		log.Printf("open backup file failed: %v", err)
		writeJSONResponseMessage(w, "读取备份快照失败", 20001)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		writeJSONResponseMessage(w, "获取备份文件信息失败", 20001)
		return
	}

	downloadFilename := fmt.Sprintf("nrllink_backup_%s.sqlite3", timestamp)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadFilename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))

	if _, err := io.Copy(w, f); err != nil {
		log.Printf("stream backup file failed: %v", err)
		return
	}

	addOperatorLog("下载数据库备份 "+downloadFilename, "数据备份", u)
}
