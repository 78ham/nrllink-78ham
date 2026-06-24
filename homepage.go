package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (j *jsonapi) httpHomepageSections(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	items, err := getHomepageSections()
	if err != nil {
		log.Println("get homepage sections err:", err)
		writeJSONResponseError(w, "获取主页内容失败")
		return
	}
	if items == nil {
		items = []HomepageSection{}
	}
	writeJSONResponseItems(w, items, len(items))
}

func (j *jsonapi) httpHomepageAnnouncements(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	req.ParseForm()
	annType, _ := strconv.Atoi(req.Form.Get("type"))
	pinned := req.Form.Get("pinned") == "true"
	limit, _ := strconv.Atoi(req.Form.Get("limit"))
	page, _ := strconv.Atoi(req.Form.Get("page"))
	if limit <= 0 {
		limit = 10
	}
	if page <= 0 {
		page = 1
	}
	items, total := getHomepageAnnouncements(annType, pinned, limit, page)
	if items == nil {
		items = []HomepageAnnouncement{}
	}
	writeJSONResponseItems(w, items, total)
}

func (j *jsonapi) httpAdminHomepageSections(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	items, err := getAllHomepageSections()
	if err != nil {
		writeJSONResponseError(w, "获取板块列表失败")
		return
	}
	if items == nil {
		items = []HomepageSection{}
	}
	writeJSONResponseItems(w, items, len(items))
}

func (j *jsonapi) httpAdminHomepageSectionsUpdate(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	section := &HomepageSection{}
	if err := jsonextra.Unmarshal(result, section); err != nil {
		w.Write(ResParmErr)
		return
	}
	if err := upsertHomepageSection(section); err != nil {
		log.Println("upsert homepage section err:", err)
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(section.SectionKey, "更新主页板块", u)
	w.Write(ResOK)
}

func (j *jsonapi) httpAdminHomepageSectionsDelete(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	section := &HomepageSection{}
	if err := jsonextra.Unmarshal(result, section); err != nil {
		w.Write(ResParmErr)
		return
	}
	if err := deleteHomepageSection(section.ID); err != nil {
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(section.SectionKey, "删除主页板块", u)
	w.Write(ResOK)
}

func (j *jsonapi) httpAdminHomepageAnnouncementsCreate(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	ann := &HomepageAnnouncement{}
	if err := jsonextra.Unmarshal(result, ann); err != nil {
		w.Write(ResParmErr)
		return
	}
	if err := createHomepageAnnouncement(ann); err != nil {
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(ann.Title, "创建公告", u)
	w.Write(ResOK)
}

func (j *jsonapi) httpAdminHomepageAnnouncementsUpdate(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	ann := &HomepageAnnouncement{}
	if err := jsonextra.Unmarshal(result, ann); err != nil {
		w.Write(ResParmErr)
		return
	}
	if err := updateHomepageAnnouncement(ann); err != nil {
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(ann.Title, "更新公告", u)
	w.Write(ResOK)
}

func (j *jsonapi) httpAdminHomepageAnnouncementsDelete(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	ann := &HomepageAnnouncement{}
	if err := jsonextra.Unmarshal(result, ann); err != nil {
		w.Write(ResParmErr)
		return
	}
	if err := deleteHomepageAnnouncement(ann.ID); err != nil {
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(ann.Title, "删除公告", u)
	w.Write(ResOK)
}

func (j *jsonapi) httpAdminHomepageImageUpload(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, 10<<20)
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		w.Write(ResParmErr)
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		w.Write(ResParmErr)
		return
	}
	defer file.Close()

	uploadDir := filepath.Join(conf.Web.Path, "uploads", time.Now().Format("2006-01"))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		w.Write(ResOpErr)
		return
	}

	ext := filepath.Ext(header.Filename)
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true}
	if !allowedExts[strings.ToLower(ext)] {
		w.Write(ResParmErr)
		return
	}
	filename := strconv.FormatInt(time.Now().UnixNano(), 36) + ext
	filePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		w.Write(ResOpErr)
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		w.Write(ResOpErr)
		return
	}

	urlPath := "/uploads/" + time.Now().Format("2006-01") + "/" + filename
	img := &HomepageImage{
		Filename: header.Filename,
		URLPath:  urlPath,
		FileSize: int(written),
	}
	if err := insertHomepageImage(img); err != nil {
		log.Println("insert homepage image err:", err)
	}
	addOperatorLog(header.Filename, "上传图片", u)
	writeJSONResponseItem(w, img)
}

func (j *jsonapi) httpAdminHomepageImageList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	items, err := getHomepageImages()
	if err != nil {
		writeJSONResponseError(w, "获取图片列表失败")
		return
	}
	if items == nil {
		items = []HomepageImage{}
	}
	writeJSONResponseItems(w, items, len(items))
}

func (j *jsonapi) httpAdminHomepageImageDelete(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	img := &HomepageImage{}
	if err := jsonextra.Unmarshal(result, img); err != nil {
		w.Write(ResParmErr)
		return
	}
	if err := deleteHomepageImage(img.ID); err != nil {
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(img.Filename, "删除图片", u)
	w.Write(ResOK)
}
