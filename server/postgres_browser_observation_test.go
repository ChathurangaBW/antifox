package server

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"antifox/db"
	"golang.org/x/crypto/bcrypt"
)

// Explicit opt-in: the runner supplies a disposable database, never the app DSN.
// Only real read handlers and rejected controls are exposed; no engine is built.
func TestDisposablePostgresBrowser(t *testing.T) {
	dsn := os.Getenv("ANTIFOX_DISPOSABLE_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL integration runner")
	}
	if !strings.Contains(dsn, "/antifox_disposable") {
		t.Fatal("disposable database name required")
	}
	admin, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	_, err = admin.Exec(`
INSERT INTO rbac_users(id,username,password_hash) VALUES ('fixture','fixture','unused');
INSERT INTO rbac_roles(id,name,scope) VALUES ('fixture','fixture','all');
INSERT INTO rbac_user_roles VALUES ('fixture','fixture');
INSERT INTO rbac_role_permissions VALUES ('fixture','dashboard:read'),('fixture','task:read'),('fixture','task:write');
CREATE ROLE observation_reader LOGIN PASSWORD 'disposable-reader';
GRANT CONNECT ON DATABASE antifox_disposable TO observation_reader;
GRANT USAGE ON SCHEMA public TO observation_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO observation_reader;`)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("pgx", os.Getenv("ANTIFOX_DISPOSABLE_READER_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	pg := &db.DB{DB: conn}
	if err := pg.Ping(); err != nil {
		t.Fatal(err)
	}
	var canWrite bool
	if err := pg.QueryRow(`SELECT has_table_privilege(current_user, 'tasks', 'UPDATE')`).Scan(&canWrite); err != nil || canWrite {
		t.Fatalf("reader permissions: write=%v err=%v", canWrite, err)
	}
	task := &Task{ID: "5", ExpID: 5, Status: "created", Store: pg.Exploration(5), FirstRunAt: 100, DeadlineAt: 700}
	m := &Manager{pg: pg, assets: pg.Assets(), tasks: map[string]*Task{"5": task}, active: "5"}
	s := &Server{m: m, engine: &Engine{}, jwtKey: []byte("disposable-browser-fixture-key")}
	token, err := signJWT(s.jwtKey, "fixture", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	login := &Server{m: &Manager{pg: admin}, jwtKey: s.jwtKey, loginLimiter: newLoginLimiter()}
	password := "Isolated-Fixture-Password-42!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`UPDATE rbac_users SET password_hash=$1 WHERE id='fixture'`, string(hash)); err != nil {
		t.Fatal(err)
	}
	mux.HandleFunc("POST /api/auth/login", login.authLogin)
	mux.HandleFunc("GET /api/auth/status", login.authStatus)
	mux.HandleFunc("GET /api/rbac/me", s.rbacMe)
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("POST /api/tasks/{id}/control", s.control)
	protected := s.requireAuth(mux)
	ui := s.webuiHandler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__api_fixture" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!doctype html><title>API fixture</title>"))
			return
		}
		if os.Getenv("ANTIFOX_TEST_ACTUAL_UI") == "1" && !strings.HasPrefix(r.URL.Path, "/api/") {
			ui.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!doctype html><title>Isolated API browser fixture</title>"))
			return
		}
		protected.ServeHTTP(w, r)
	}))
	defer server.Close()
	if os.Getenv("ANTIFOX_TEST_ACTUAL_UI") == "1" {
		uiScript := `const {chromium}=require(process.env.PLAYWRIGHT_PATH);
(async()=>{const b=await chromium.launch({headless:true,executablePath:process.env.TEST_CHROMIUM_PATH,args:['--no-sandbox']});try{
const p=await b.newPage();const errors=[],external=[],logins=[];p.on('pageerror',e=>errors.push(e.message));
await p.addInitScript(()=>localStorage.setItem('antifox-lang','en-US'));
await p.route('**/*',r=>{if(new URL(r.request().url()).origin!==process.env.FIXTURE_URL){external.push(r.request().url());return r.abort()}return r.continue()});
p.on('response',r=>{if(r.url().endsWith('/api/auth/login'))logins.push(r.status())});
await p.goto(process.env.FIXTURE_URL+'/login/');await p.locator('#username').fill('fixture');await p.locator('#password').fill('incorrect-password');
await Promise.all([p.waitForResponse(r=>r.url().endsWith('/api/auth/login')&&r.status()===401),p.locator('button[type=submit]').click()]);
if(await p.evaluate(()=>localStorage.getItem('antifox_token')))throw Error('invalid password stored token');
await p.locator('#password').fill(process.env.FIXTURE_PASSWORD);
await Promise.all([p.waitForResponse(r=>r.url().endsWith('/api/auth/login')&&r.status()===200),p.locator('button[type=submit]').click()]);
await p.waitForURL('**/function/tasks/**');await p.getByText('UI login fixture task',{exact:true}).waitFor();
if(!await p.evaluate(()=>localStorage.getItem('antifox_token')))throw Error('login token missing');
await p.reload();await p.getByText('UI login fixture task',{exact:true}).waitFor();
if(errors.length||external.length)throw Error(JSON.stringify({errors,external}));
await p.screenshot({path:process.env.FIXTURE_SCREENSHOT,fullPage:true});console.log(JSON.stringify({loginStatuses:logins,taskVisible:true,reloadAuthenticated:true,javascriptErrors:errors,externalRequests:external}));
}finally{await b.close()}})().catch(e=>{console.error(e);process.exit(1)});`
		task.Name = "UI login fixture task"
		task.Description = "UI login fixture task"
		file := filepath.Join(t.TempDir(), "ui-login.cjs")
		if err := os.WriteFile(file, []byte(uiScript), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("node", file)
		cmd.Env = append(os.Environ(), "FIXTURE_URL="+server.URL, "FIXTURE_PASSWORD="+password)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("actual UI login: %v: %s", err, out)
		}
		t.Logf("actual UI login: %s", out)
	}
	script := `const {chromium}=require(process.env.PLAYWRIGHT_PATH);
(async()=>{const browser=await chromium.launch({headless:true,executablePath:process.env.TEST_CHROMIUM_PATH,args:['--no-sandbox']});
try { const page=await browser.newPage(); const base=process.env.FIXTURE_URL;
await page.route('**/*',r=>new URL(r.request().url()).origin===base?r.continue():r.abort());
await page.goto(base+'/__api_fixture'); const results=await page.evaluate(async ({token,phase})=>{
const results=[]; for(const [path,method,auth,want] of [
['/api/stats?task=5','GET',true,phase==='healthy'?200:503],
['/api/tasks','GET',true,phase==='auth_failure'?503:200],
['/api/tasks/5/control','POST',true,phase==='auth_failure'?503:409],
['/api/stats','GET',false,401]]) {
const response=await fetch(path,{method,headers:auth?{Authorization:'Bearer '+token}:{},...(method==='POST'?{body:JSON.stringify({action:'rerun'})}:{})});
const body=await response.text(); if(response.status!==want)throw Error(path+': '+response.status+' '+body);
results.push({path,status:response.status}); } return results;
},{token:process.env.FIXTURE_TOKEN,phase:process.env.FIXTURE_PHASE}); console.log(JSON.stringify(results));
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exit(1)});`
	file := filepath.Join(t.TempDir(), "browser.cjs")
	if err := os.WriteFile(file, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"healthy", "telemetry_failure", "auth_failure"} {
		if phase == "telemetry_failure" {
			_, err = admin.Exec(`REVOKE SELECT ON assets FROM observation_reader`)
		}
		if phase == "auth_failure" {
			_, err = admin.Exec(`REVOKE SELECT ON rbac_users FROM observation_reader`)
		}
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("node", file)
		cmd.Env = append(os.Environ(), "FIXTURE_URL="+server.URL, "FIXTURE_TOKEN="+token, "FIXTURE_PHASE="+phase)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v: %s", phase, err, out)
		}
		t.Logf("browser %s: %s", phase, out)
	}
	if task.Status != "created" || task.FirstRunAt != 100 || task.DeadlineAt != 700 || task.CompletedAt != 0 || task.Paused {
		t.Fatal("task state changed")
	}
	t.Log("PASS: real PostgreSQL, read-only runtime role, browser JWT requests; inert engine and preserved clocks")
}
