package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func setupAdminTest(t *testing.T) string {
	t.Helper()
	testDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldTokenKey, oldLimit := db, TokenKey, conf.Security.MaxLoginAttempts
	db, TokenKey, conf.Security.MaxLoginAttempts = testDB, []byte("local-test-signing-key"), 10
	t.Cleanup(func() {
		testDB.Close()
		db, TokenKey, conf.Security.MaxLoginAttempts = oldDB, oldTokenKey, oldLimit
		userlist.Clear()
		userPhoneIndex.Clear()
		mdcidmap.Clear()
		dmridmap.Clear()
	})
	execDDL()
	updatedb()
	admin := userinfo{Name: "Admin", CallSign: "TESTADMIN", Phone: "10000000001", Roles: []string{"admin"}, Status: 1}
	if err := addUser(&admin, "Fixture!1234"); err != nil {
		t.Fatal(err)
	}
	token, err := GenerateToken(admin.CallSign, admin.Roles, 1)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func panelRequest(t *testing.T, handler http.HandlerFunc, method, token, body string, wantCode int) json.RawMessage {
	t.Helper()
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("x-token", token)
	w := httptest.NewRecorder()
	handler(w, req)
	var response struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != wantCode {
		t.Fatalf("expected code %d, got %d: %s", wantCode, response.Code, response.Data)
	}
	return response.Data
}

func TestPanelUserCredentials(t *testing.T) {
	admin := setupAdminTest(t)
	panelRequest(t, jsonhttp.httpAddUser, "POST", admin,
		`{"name":"Demo","callsign":"TESTUSER","phone":"10000000002","status":1,"roles":["ham"],"password":"Initial!1234"}`, 20000)
	user, err := getUserCached("TESTUSER")
	if err != nil || user.ID == 0 || user.TokenVersion != 1 {
		t.Fatalf("created user did not receive the persisted identity: %v", err)
	}
	if _, ok := loginCheck("Initial!1234", user.CallSign, "localhost"); !ok {
		t.Fatal("created user could not log in")
	}
	token, err := GenerateToken(user.CallSign, user.Roles, user.TokenVersion)
	if err != nil {
		t.Fatal(err)
	}
	data := panelRequest(t, jsonhttp.httpUserInfo, "GET", token, "", 20000)
	var profile map[string]json.RawMessage
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if _, found := profile["password"]; found {
		t.Fatal("password included in profile response")
	}
	id := string(profile["id"])
	panelRequest(t, jsonhttp.httpUpdateUserProfile, "POST", token, `{"id":`+id+`,"mdcid":"ABCD","password":"short"}`, 20001)
	if stored, err := getuserByID(user.ID); err != nil || stored.MDCID != "" {
		t.Fatalf("invalid profile partially persisted: %v", err)
	}
	panelRequest(t, jsonhttp.httpUpdateUserProfile, "POST", token, `{"id":`+id+`,"mdcid":"1234"}`, 20000)
	panelRequest(t, jsonhttp.httpUpdateUserPassword, "POST", token, `{"password":""}`, 20001)
	panelRequest(t, jsonhttp.httpUpdateUserPassword, "POST", token, `{"password":"Changed!1234"}`, 20000)
	if _, ok := loginCheck("Changed!1234", user.CallSign, "localhost"); !ok {
		t.Fatal("updated password did not work")
	}
	current, err := getUserCached(user.Phone)
	if err != nil || current.ID != user.ID || current.TokenVersion != 2 || current.MDCID != "1234" {
		t.Fatalf("phone index was not refreshed: %v", err)
	}
	panelRequest(t, jsonhttp.httpUpdateUser, "POST", admin, `{"id":`+id+`,"name":"Renamed","callsign":"TESTNEW","phone":"10000000003","status":1,"roles":["ham"],"password":"Managed!1234"}`, 20000)
	if _, ok := loginCheck("Managed!1234", "TESTNEW", "localhost"); !ok {
		t.Fatal("administrator password change failed")
	}
	if _, exists := userlist.Load(user.CallSign); exists {
		t.Fatal("old callsign remained cached")
	}
	if _, exists := userPhoneIndex.Load(user.Phone); exists {
		t.Fatal("old phone remained cached")
	}
}

func TestPanelBMCredentials(t *testing.T) {
	admin := setupAdminTest(t)
	initBMNetworkTables()
	list := panelRequest(t, jsonhttp.httpBMNetworkList, "GET", admin, "", 20000)
	if strings.Contains(string(list), `"password"`) {
		t.Fatal("network list returned a password")
	}
	const body = `{"id":1,"name":"Edited","server_address":"localhost"`
	for _, tc := range []struct{ field, password string }{
		{"", "pass123456"},
		{`,"password":"changed"`, "changed"},
		{`,"password":""`, ""},
	} {
		panelRequest(t, jsonhttp.httpBMNetworkUpdate, "POST", admin, body+tc.field+"}", 20000)
		n, err := getBMNetworkByID(1)
		if err != nil || n.Password != tc.password || n.Name != "Edited" {
			t.Fatalf("unexpected stored BM configuration: %v", err)
		}
	}
}

func TestPanelRoutesAndLogout(t *testing.T) {
	admin := setupAdminTest(t)
	if got := panelRequest(t, jsonhttp.httpGetRoutes, "GET", admin, "", 20000); string(got) != "[]" {
		t.Fatalf("initial routes = %s", got)
	}
	const routes = `[{"path":"/setup"}]`
	panelRequest(t, jsonhttp.httpSetRoutes, "POST", admin, routes, 20000)
	if got := panelRequest(t, jsonhttp.httpGetRoutes, "GET", admin, "", 20000); string(got) != routes {
		t.Fatalf("saved routes = %s", got)
	}
	panelRequest(t, jsonhttp.httpSetRoutes, "POST", admin, "[]", 20000)
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM routes").Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected one settings row, got %d: %v", count, err)
	}
	panelRequest(t, jsonhttp.httpoplogout, "POST", admin, "", 20000)
	u, err := getUserCached("TESTADMIN")
	if err != nil || u.TokenVersion != 2 {
		t.Fatalf("empty logout did not expire session: %v", err)
	}
}

func TestRoleWrites(t *testing.T) {
	setupAdminTest(t)
	r := role{NameKey: "observer", Name: "Observer"}
	if err := addRole(&r); err != nil || r.ID < 1 {
		t.Fatalf("missing persisted role identity: %v", err)
	}
	r.Name = "Observer 2"
	if err := updateRole(&r); err != nil {
		t.Fatal(err)
	}
	if err := delRole(r.NameKey); err != nil {
		t.Fatal(err)
	}
	if err := updateRole(&r); err == nil {
		t.Fatal("missing role reported a successful update")
	}
	if err := delRole("admin"); err == nil {
		t.Fatal("protected role reported a successful delete")
	}
}

func TestConcurrentPanelSessionUpdates(t *testing.T) {
	setupAdminTest(t)
	if _, err := db.Exec("UPDATE users SET last_login_ip=NULL, must_change_pwd=NULL, avatar=NULL WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	const updates = 8
	results := make(chan error, updates)
	for i := 0; i < updates; i++ {
		go func() { results <- revokeUserToken(1) }()
	}
	for i := 0; i < updates; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	for _, username := range []string{"TESTADMIN", "10000000001"} {
		u, err := getUserCached(username)
		if err != nil || u.TokenVersion != 1+updates {
			t.Fatalf("session cache out of sync for %s: %v", username, err)
		}
	}
}
