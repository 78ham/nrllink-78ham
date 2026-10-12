package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var regCallsignRegex = regexp.MustCompile(`^[A-Za-z0-9-]{3,16}$`)

func (j *jsonapi) httpRegisterList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("reg user list err :", err)
		w.Write(ResParmErr)
		return
	}

	ww, args, pp, rsort := queryToWhere("", *stb)
	emplist, total := selectReguser(ww, args, pp, rsort)

	rescode, _ := jsonextra.Marshal(emplist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		total, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpRegisterImage(w http.ResponseWriter, req *http.Request) {

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

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("reg user list err :", err)
		w.Write(ResParmErr)
		return
	}

	// 验证图片路径合法性：仅允许读取执照目录下的文件
	absPath, err := filepath.Abs(stb.Path)
	if err != nil {
		http.Error(w, "Invalid image path", http.StatusBadRequest)
		return
	}

	allowedDir, _ := filepath.Abs(conf.System.LicensePath)
	// 防止路径遍历：确保绝对路径在allowedDir下
	if !strings.HasPrefix(absPath, allowedDir+string(os.PathSeparator)) && absPath != allowedDir {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// 检查文件是否存在并读取内容
	file, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// 读取文件内容
	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// 若文件为 AES-256-GCM 加密存储，执行解密
	if isEncryptedLicense(imageData) {
		if len(licenseAESKey) == 32 {
			decrypted, err := decryptAESGCM(licenseAESKey, imageData)
			if err != nil {
				log.Println("decrypt license file failed:", err)
				http.Error(w, "Failed to decrypt license file", http.StatusInternalServerError)
				return
			}
			imageData = decrypted
		} else {
			log.Println("encrypted license detected but LicenseEncKey not configured")
			http.Error(w, "License encryption key not configured", http.StatusInternalServerError)
			return
		}
	}

	// 将二进制数据转换为 Base64 编码
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	// 构造 JSON 响应
	response := Response{
		Code:    20000,
		Message: "Image data retrieved successfully",
		Data:    base64Data,
	}

	// 设置响应头信息
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 返回 JSON 响应
	json.NewEncoder(w).Encode(response)

}

func (j *jsonapi) httpRegister(w http.ResponseWriter, r *http.Request) {

	sethttphead(w)

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 限制上传文件的大小（这里限制为 10 MB）
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	// 解析表单数据（解析时会检查大小限制）
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 这里的 10 << 20 同样是 10 MB 限制
		w.Write(ResParmErr)
		return
	}

	// 获取用户注册信息
	callsign := strings.ToUpper(strings.TrimSpace(r.FormValue("callsign")))
	name := strings.TrimSpace(r.FormValue("name"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	address := strings.TrimSpace(r.FormValue("address"))
	mail := strings.TrimSpace(r.FormValue("mail"))
	password := r.FormValue("password")

	// 检查必填字段
	if callsign == "" || name == "" || phone == "" || password == "" {
		w.Write(ResParmErr)
		return
	}

	// 校验呼号格式，阻断路径穿越和特殊字符
	if !regCallsignRegex.MatchString(callsign) {
		writeJSONResponseMessage(w, "呼号格式不合法，仅允许3-16位字母、数字或连字符", 20001)
		return
	}

	// 强制校验注册密码复杂度与手机号/呼号相似度
	if err := validatePassword(password, callsign, phone); err != nil {
		writeJSONResponseMessage(w, err.Error(), 20001)
		return
	}

	user, _ := getuser(phone)

	// if err != nil {
	// 	w.Write(ResOpErr)
	// }

	if user != nil {
		w.Write(ResUserAleadyExits)
		return
	}

	// 创建上传目录（按年月分割）
	currentMonth := time.Now().Format("2006-01")
	uploadDir := filepath.Join(conf.System.LicensePath, currentMonth)
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		w.Write(ResOpErr)
		return
	}

	// 保存文件路径
	var licensePath string

	// // 处理上传的操作证文件
	// opCertFile, opCertHeader, err := r.FormFile("certificate")
	// if err == nil {
	// 	defer opCertFile.Close()
	// 	opCertPath = filepath.Join(uploadDir, callsign+"_certificate"+filepath.Ext(opCertHeader.Filename))
	// 	if err := saveFile(opCertFile, opCertPath); err != nil {
	// 		w.Write(ResOpErr)
	// 		return
	// 	}
	// }

	// 处理上传的电台执照文件
	licenseFile, licenseHeader, err := r.FormFile("license")
	if err == nil && licenseHeader != nil {
		defer licenseFile.Close()
		ext := strings.ToLower(filepath.Ext(licenseHeader.Filename))
		allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true}
		if !allowedExts[ext] {
			writeJSONResponseMessage(w, "执照图片格式仅支持 jpg/jpeg/png", 20001)
			return
		}
		licensePath = filepath.Join(uploadDir, callsign+"_license"+ext)
		if err := saveFile(licenseFile, licensePath); err != nil {
			log.Println("注册信息licensePath：", err)
			w.Write(ResOpErr)
			return
		}
	}

	//fmt.Println("opCertPath:", opCertPath)
	fmt.Println("licensePath:", licensePath)

	reguser := &reguser{
		CallSign:    callsign,
		Name:        name,
		Phone:       phone,
		Address:     address,
		Mail:        mail,
		Password:    password,
		OpCertPath:  licensePath,
		LicensePath: licensePath,
		CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
		UpdateTime:  time.Now().Format("2006-01-02 15:04:05"),
		Status:      1,
	}

	fmt.Println("注册信息：", reguser)

	err = createRegUser(reguser)

	if err != nil {
		log.Println("add reg user failed  , ", err)
		w.Write(ResParmErr)
		return

	}

	// 返回注册成功信息
	w.Write(ResOK)
}

func (j *jsonapi) httpAddReg(w http.ResponseWriter, req *http.Request) {

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

	stb := &reguser{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("query err :", err)

		w.Write(ResParmErr)
		return

	}

	stb.Note = u.Name + u.CallSign

	err = addUserReg(req.Context(), stb)
	if err != nil {
		log.Println("area school add err :", err)
		//writeJSONResponse(w, &Response{20001,  "校区添加失败", nil))
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(stb.Name+stb.CallSign+stb.Phone, "增加用户", u)

	//writeJSONResponse(w, &Response{20000, "校区添加成功", nil))
	w.Write(ResOK)

}

func (j *jsonapi) httpUpdateReg(w http.ResponseWriter, req *http.Request) {
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

	stb := &reguser{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("query err :", err)

		w.Write(ResParmErr)
		return

	}

	err = updateRegUser(stb)

	if err != nil {
		w.Write(ResOpErr)
		return

	}

	w.Write(ResOK)

}

func (j *jsonapi) httpDeleteRegUser(w http.ResponseWriter, req *http.Request) {
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

	stb := &reguser{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user delete err :", err)
		w.Write(ResOpErr)
		return
	}

	deleteRegUser(stb)
	addOperatorLog(stb.CallSign, "用户删除成功", u)

	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"用户删除成功"}}`))

}
