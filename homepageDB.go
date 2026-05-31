package main

import (
	"log"
	"time"
)

type HomepageSection struct {
	ID        int    `json:"id" db:"id"`
	SectionKey string `json:"section_key" db:"section_key"`
	Title     string `json:"title" db:"title"`
	Subtitle  string `json:"subtitle" db:"subtitle"`
	Content   string `json:"content" db:"content"`
	Type      int    `json:"type" db:"type"`
	SortOrder int    `json:"sort_order" db:"sort_order"`
	Status    int    `json:"status" db:"status"`
	Extra     string `json:"extra" db:"extra"`
	CreatedAt string `json:"create_time" db:"create_time"`
	UpdatedAt string `json:"update_time" db:"update_time"`
}

type HomepageAnnouncement struct {
	ID          int    `json:"id" db:"id"`
	Title       string `json:"title" db:"title"`
	Summary     string `json:"summary" db:"summary"`
	Content     string `json:"content" db:"content"`
	CoverURL    string `json:"cover_url" db:"cover_url"`
	Type        int    `json:"type" db:"type"`
	IsPinned    int    `json:"is_pinned" db:"is_pinned"`
	IsPublished int    `json:"is_published" db:"is_published"`
	PublishTime string `json:"publish_time" db:"publish_time"`
	CreatedAt   string `json:"create_time" db:"create_time"`
	UpdatedAt   string `json:"update_time" db:"update_time"`
}

type HomepageImage struct {
	ID        int    `json:"id" db:"id"`
	Filename  string `json:"filename" db:"filename"`
	URLPath   string `json:"url_path" db:"url_path"`
	FileSize  int    `json:"file_size" db:"file_size"`
	Width     int    `json:"width" db:"width"`
	Height    int    `json:"height" db:"height"`
	AltText   string `json:"alt_text" db:"alt_text"`
	Category  string `json:"category" db:"category"`
	CreatedAt string `json:"create_time" db:"create_time"`
}

func initHomepageTables() {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS homepage_sections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			section_key TEXT NOT NULL UNIQUE,
			title TEXT DEFAULT '',
			subtitle TEXT DEFAULT '',
			content TEXT DEFAULT '',
			type INTEGER DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			status INTEGER DEFAULT 1,
			extra TEXT DEFAULT '{}',
			create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			update_time DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS homepage_announcements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			summary TEXT DEFAULT '',
			content TEXT DEFAULT '',
			cover_url TEXT DEFAULT '',
			type INTEGER DEFAULT 1,
			is_pinned INTEGER DEFAULT 0,
			is_published INTEGER DEFAULT 0,
			publish_time DATETIME,
			create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			update_time DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS homepage_images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT NOT NULL,
			url_path TEXT NOT NULL,
			file_size INTEGER DEFAULT 0,
			width INTEGER DEFAULT 0,
			height INTEGER DEFAULT 0,
			alt_text TEXT DEFAULT '',
			category TEXT DEFAULT 'general',
			create_time DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range statements {
		var applied int
		if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE stmt=?`, stmt).Scan(&applied); err != nil {
			log.Printf("check homepage migration err: %v", err)
			continue
		}
		if applied > 0 {
			continue
		}

		_, err := db.Exec(stmt)
		if err != nil {
			log.Printf("create homepage table err: %v", err)
		}
		if _, err := db.Exec(`INSERT OR IGNORE INTO schema_migrations(stmt, applied_at) VALUES(?, CURRENT_TIMESTAMP)`, stmt); err != nil {
			log.Printf("record homepage migration err: %v", err)
		}
	}
}

func getHomepageSections() ([]HomepageSection, error) {
	items := []HomepageSection{}
	rows, err := db.Query(`SELECT id, section_key, title, subtitle, content, type, sort_order, status, extra, create_time, update_time FROM homepage_sections WHERE status=1 ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item HomepageSection
		if err := rows.Scan(&item.ID, &item.SectionKey, &item.Title, &item.Subtitle, &item.Content, &item.Type, &item.SortOrder, &item.Status, &item.Extra, &item.CreatedAt, &item.UpdatedAt); err != nil {
			log.Println("scan homepage_section err:", err)
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func getAllHomepageSections() ([]HomepageSection, error) {
	items := []HomepageSection{}
	rows, err := db.Query(`SELECT id, section_key, title, subtitle, content, type, sort_order, status, extra, create_time, update_time FROM homepage_sections ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item HomepageSection
		if err := rows.Scan(&item.ID, &item.SectionKey, &item.Title, &item.Subtitle, &item.Content, &item.Type, &item.SortOrder, &item.Status, &item.Extra, &item.CreatedAt, &item.UpdatedAt); err != nil {
			log.Println("scan homepage_section err:", err)
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func upsertHomepageSection(section *HomepageSection) error {
	_, err := db.Exec(`INSERT INTO homepage_sections (section_key, title, subtitle, content, type, sort_order, status, extra, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(section_key) DO UPDATE SET
		title=excluded.title, subtitle=excluded.subtitle, content=excluded.content,
		type=excluded.type, sort_order=excluded.sort_order, status=excluded.status,
		extra=excluded.extra, update_time=CURRENT_TIMESTAMP`,
		section.SectionKey, section.Title, section.Subtitle, section.Content, section.Type, section.SortOrder, section.Status, section.Extra)
	return err
}

func deleteHomepageSection(id int) error {
	_, err := db.Exec(`DELETE FROM homepage_sections WHERE id=?`, id)
	return err
}

func getHomepageAnnouncements(annType int, pinned bool, limit, page int) ([]HomepageAnnouncement, int) {
	items := []HomepageAnnouncement{}
	where := "WHERE is_published=1"
	args := []interface{}{}
	if annType > 0 {
		where += " AND type=?"
		args = append(args, annType)
	}
	if pinned {
		where += " AND is_pinned=1"
	}

	var total int
	db.QueryRow(`SELECT count(*) FROM homepage_announcements `+where, args...).Scan(&total)

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := db.Query(`SELECT id, title, summary, content, cover_url, type, is_pinned, is_published, publish_time, create_time, update_time
		FROM homepage_announcements `+where+` ORDER BY is_pinned DESC, publish_time DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		log.Println("getHomepageAnnouncements err:", err)
		return items, 0
	}
	defer rows.Close()

	for rows.Next() {
		var item HomepageAnnouncement
		if err := rows.Scan(&item.ID, &item.Title, &item.Summary, &item.Content, &item.CoverURL, &item.Type, &item.IsPinned, &item.IsPublished, &item.PublishTime, &item.CreatedAt, &item.UpdatedAt); err != nil {
			log.Println("scan homepage_announcement err:", err)
			continue
		}
		items = append(items, item)
	}
	return items, total
}

func getHomepageAnnouncementByID(id int) (*HomepageAnnouncement, error) {
	item := &HomepageAnnouncement{}
	row := db.QueryRow(`SELECT id, title, summary, content, cover_url, type, is_pinned, is_published, publish_time, create_time, update_time
		FROM homepage_announcements WHERE id=?`, id)
	err := row.Scan(&item.ID, &item.Title, &item.Summary, &item.Content, &item.CoverURL, &item.Type, &item.IsPinned, &item.IsPublished, &item.PublishTime, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func createHomepageAnnouncement(ann *HomepageAnnouncement) error {
	if ann.PublishTime == "" {
		ann.PublishTime = time.Now().UTC().Format("2006-01-02 15:04:05")
	}
	_, err := db.Exec(`INSERT INTO homepage_announcements (title, summary, content, cover_url, type, is_pinned, is_published, publish_time, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		ann.Title, ann.Summary, ann.Content, ann.CoverURL, ann.Type, ann.IsPinned, ann.IsPublished, ann.PublishTime)
	return err
}

func updateHomepageAnnouncement(ann *HomepageAnnouncement) error {
	_, err := db.Exec(`UPDATE homepage_announcements SET title=?, summary=?, content=?, cover_url=?, type=?, is_pinned=?, is_published=?, publish_time=?, update_time=CURRENT_TIMESTAMP WHERE id=?`,
		ann.Title, ann.Summary, ann.Content, ann.CoverURL, ann.Type, ann.IsPinned, ann.IsPublished, ann.PublishTime, ann.ID)
	return err
}

func deleteHomepageAnnouncement(id int) error {
	_, err := db.Exec(`DELETE FROM homepage_announcements WHERE id=?`, id)
	return err
}

func insertHomepageImage(img *HomepageImage) error {
	_, err := db.Exec(`INSERT INTO homepage_images (filename, url_path, file_size, width, height, alt_text, category, create_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		img.Filename, img.URLPath, img.FileSize, img.Width, img.Height, img.AltText, img.Category)
	return err
}

func getHomepageImages() ([]HomepageImage, error) {
	items := []HomepageImage{}
	rows, err := db.Query(`SELECT id, filename, url_path, file_size, width, height, alt_text, category, create_time FROM homepage_images ORDER BY create_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item HomepageImage
		if err := rows.Scan(&item.ID, &item.Filename, &item.URLPath, &item.FileSize, &item.Width, &item.Height, &item.AltText, &item.Category, &item.CreatedAt); err != nil {
			log.Println("scan homepage_image err:", err)
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func deleteHomepageImage(id int) error {
	_, err := db.Exec(`DELETE FROM homepage_images WHERE id=?`, id)
	return err
}
