package main

import (
	"database/sql"
	"log"
	"time"
)

type BMNetwork struct {
	ID                int    `json:"id" db:"id"`
	Name              string `json:"name" db:"name"`
	ServerAddress     string `json:"server_address" db:"server_address"`
	ServerPort        int    `json:"server_port" db:"server_port"`
	Password          string `json:"password" db:"password"`
	Callsign          string `json:"callsign" db:"callsign"`
	DMRID             int    `json:"dmrid" db:"dmrid"`
	DefaultTG         int    `json:"default_tg" db:"default_tg"`
	Timeslot          int    `json:"timeslot" db:"timeslot"`
	HeartbeatInterval int    `json:"heartbeat_interval" db:"heartbeat_interval"`
	Status            int    `json:"status" db:"status"`
	Note              string `json:"note" db:"note"`
	CreateTime        string `json:"create_time" db:"create_time"`
	UpdateTime        string `json:"update_time" db:"update_time"`
}

type BMBridge struct {
	ID            int     `json:"id" db:"id"`
	NetworkID     int     `json:"network_id" db:"network_id"`
	DeviceID      int     `json:"device_id" db:"device_id"`
	Status        int     `json:"status" db:"status"`
	RxPackets     int64   `json:"rx_packets" db:"rx_packets"`
	TxPackets     int64   `json:"tx_packets" db:"tx_packets"`
	LossRate      float64 `json:"loss_rate" db:"loss_rate"`
	LastHeartbeat string  `json:"last_heartbeat" db:"last_heartbeat"`
	ErrorMsg      string  `json:"error_msg" db:"error_msg"`
	CreateTime    string  `json:"create_time" db:"create_time"`
	UpdateTime    string  `json:"update_time" db:"update_time"`
}

func initBMNetworkTables() {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS bm_networks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			server_address TEXT NOT NULL DEFAULT '',
			server_port INTEGER NOT NULL DEFAULT 62031,
			password TEXT NOT NULL DEFAULT '',
			callsign TEXT NOT NULL DEFAULT '',
			dmrid INTEGER NOT NULL DEFAULT 0,
			default_tg INTEGER NOT NULL DEFAULT 46001,
			timeslot INTEGER NOT NULL DEFAULT 2,
			heartbeat_interval INTEGER NOT NULL DEFAULT 10,
			status INTEGER NOT NULL DEFAULT 1,
			note TEXT DEFAULT '',
			create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			update_time DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bm_bridges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			network_id INTEGER NOT NULL,
			device_id INTEGER NOT NULL UNIQUE,
			status INTEGER NOT NULL DEFAULT 0,
			rx_packets INTEGER DEFAULT 0,
			tx_packets INTEGER DEFAULT 0,
			loss_rate REAL DEFAULT 0.0,
			last_heartbeat DATETIME DEFAULT CURRENT_TIMESTAMP,
			error_msg TEXT DEFAULT '',
			create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			update_time DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			log.Printf("[bm] DDL error: %v\n  SQL: %s", err, stmt)
		}
	}

	var count int
	if err := db.QueryRow("SELECT count(*) FROM bm_networks").Scan(&count); err == nil && count == 0 {
		_, _ = db.Exec(`INSERT INTO bm_networks 
			(name, server_address, server_port, password, callsign, dmrid, default_tg, timeslot, heartbeat_interval, status, note)
			VALUES ('BrandMeister 4601 (Master)', 'bm.4601.master', 62031, 'pass123456', 'NOCALL', 4600000, 46001, 2, 10, 1, '默认 BrandMeister 4601 节点')`)
	}
}

func getBMNetworkList() ([]BMNetwork, error) {
	rows, err := db.Query("SELECT id, name, server_address, server_port, password, callsign, dmrid, default_tg, timeslot, heartbeat_interval, status, note, create_time, update_time FROM bm_networks ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []BMNetwork
	for rows.Next() {
		var n BMNetwork
		if err := rows.Scan(
			&n.ID, &n.Name, &n.ServerAddress, &n.ServerPort, &n.Password,
			&n.Callsign, &n.DMRID, &n.DefaultTG, &n.Timeslot, &n.HeartbeatInterval,
			&n.Status, &n.Note, &n.CreateTime, &n.UpdateTime,
		); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, nil
}

func getBMNetworkByID(id int) (*BMNetwork, error) {
	var n BMNetwork
	err := db.QueryRow("SELECT id, name, server_address, server_port, password, callsign, dmrid, default_tg, timeslot, heartbeat_interval, status, note, create_time, update_time FROM bm_networks WHERE id=?", id).Scan(
		&n.ID, &n.Name, &n.ServerAddress, &n.ServerPort, &n.Password,
		&n.Callsign, &n.DMRID, &n.DefaultTG, &n.Timeslot, &n.HeartbeatInterval,
		&n.Status, &n.Note, &n.CreateTime, &n.UpdateTime,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func insertBMNetwork(n *BMNetwork) (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	res, err := db.Exec(`INSERT INTO bm_networks
		(name, server_address, server_port, password, callsign, dmrid, default_tg, timeslot, heartbeat_interval, status, note, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Name, n.ServerAddress, n.ServerPort, n.Password, n.Callsign, n.DMRID,
		n.DefaultTG, n.Timeslot, n.HeartbeatInterval, n.Status, n.Note, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateBMNetwork(n *BMNetwork) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`UPDATE bm_networks SET
		name=?, server_address=?, server_port=?, password=?, callsign=?, dmrid=?,
		default_tg=?, timeslot=?, heartbeat_interval=?, status=?, note=?, update_time=?
		WHERE id=?`,
		n.Name, n.ServerAddress, n.ServerPort, n.Password, n.Callsign, n.DMRID,
		n.DefaultTG, n.Timeslot, n.HeartbeatInterval, n.Status, n.Note, now, n.ID,
	)
	return err
}

func deleteBMNetwork(id int) error {
	_, err := db.Exec("DELETE FROM bm_networks WHERE id=?", id)
	return err
}

func getBMBridgeByDevice(deviceID int) (*BMBridge, error) {
	var b BMBridge
	err := db.QueryRow("SELECT id, network_id, device_id, status, rx_packets, tx_packets, loss_rate, last_heartbeat, error_msg, create_time, update_time FROM bm_bridges WHERE device_id=?", deviceID).Scan(
		&b.ID, &b.NetworkID, &b.DeviceID, &b.Status, &b.RxPackets, &b.TxPackets,
		&b.LossRate, &b.LastHeartbeat, &b.ErrorMsg, &b.CreateTime, &b.UpdateTime,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func upsertBMBridge(b *BMBridge) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`INSERT INTO bm_bridges
		(network_id, device_id, status, rx_packets, tx_packets, loss_rate, last_heartbeat, error_msg, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET
		network_id=excluded.network_id,
		status=excluded.status,
		rx_packets=excluded.rx_packets,
		tx_packets=excluded.tx_packets,
		loss_rate=excluded.loss_rate,
		last_heartbeat=excluded.last_heartbeat,
		error_msg=excluded.error_msg,
		update_time=excluded.update_time`,
		b.NetworkID, b.DeviceID, b.Status, b.RxPackets, b.TxPackets,
		b.LossRate, now, b.ErrorMsg, now, now,
	)
	return err
}
