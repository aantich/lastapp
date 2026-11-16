package control

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPasswordLengthBounds(t *testing.T) {
	for _, tc := range []struct {
		name, password string
		valid          bool
	}{
		{"too short", "1234567", false},
		{"minimum", "Abc!1234", true},
		{"unicode below minimum", strings.Repeat("界", 7), false},
		{"unicode minimum", strings.Repeat("界", 8), true},
		{"maximum", strings.Repeat("x", 128), true},
		{"too long", strings.Repeat("x", 129), false},
		{"invalid unicode", "password\xff", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if e := validatePassword(tc.password); (e == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, e)
			}
		})
	}
}

func TestPublicPasswordPolicy(t *testing.T) {
	// Public config must be available before login and without a database.
	s := &Server{mux: http.NewServeMux()}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/v1/config", nil))
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("public config response", w.Code, w.Header())
	}
	var config struct {
		Policy struct {
			Min  int    `json:"min_length"`
			Max  int    `json:"max_length"`
			Hint string `json:"length_hint"`
		} `json:"password_policy"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &config); e != nil {
		t.Fatal(e)
	}
	p := config.Policy
	if p.Min != minPasswordLength || p.Max != maxPasswordLength || p.Hint != passwordLengthHint() {
		t.Fatal("config differs from authoritative policy", p)
	}
	for _, n := range []int{p.Min - 1, p.Min, p.Max, p.Max + 1} {
		e := validatePassword(strings.Repeat("😀", n))
		if (e == nil) != (n >= p.Min && n <= p.Max) {
			t.Fatalf("public bounds disagree with Unicode validation at %d: %v", n, e)
		}
		if e != nil && !strings.Contains(e.Error(), p.Hint) {
			t.Fatal("validation message differs from public policy", e)
		}
	}
}

// A slow PBKDF2 derivation (race detector, loaded CI) must not exhaust the
// login's database deadline and turn a correct password into a server error.
func TestLoginDeadlineExcludesPasswordDerivation(t *testing.T) {
	s, platform, _, _, _ := foundationFixture(t)
	email, password := "slow-"+newID()+"@example.test", "Slow derivation 8!"
	platform.want("POST", "/admin/v1/users", Row{"display_name": "Slow derivation", "email": email, "password": password}, 0, 201)
	start := time.Now()
	passwordKey(password, make([]byte, 16), passwordIterations)
	derivation := time.Since(start)
	// Each database phase is far shorter than half a derivation; the derivation
	// itself is longer than this deadline.
	previous := loginDatabaseTimeout
	loginDatabaseTimeout = derivation / 2
	t.Cleanup(func() { loginDatabaseTimeout = previous })
	passwordSignIn(platform, email, password, 200)
	passwordSignIn(platform, email, "Wrong derivation 8!", 401)
	var sessions int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM sessions s JOIN local_accounts a ON a.user_id=s.user_id WHERE a.login_email=?", email).Scan(&sessions); e != nil || sessions != 1 {
		t.Fatal("login did not create exactly one session", sessions, e)
	}
}

// Every request that derives a new or changed password must give its database
// work a deadline that the uncancellable PBKDF2 derivation has not consumed.
func TestRequestDeadlineExcludesPasswordDerivation(t *testing.T) {
	_, platform, _, _, _ := foundationFixture(t)
	start := time.Now()
	passwordKey("Slow derivation 8!", make([]byte, 16), passwordIterations)
	derivation := time.Since(start)
	previous := requestTimeout
	requestTimeout = derivation / 2
	t.Cleanup(func() { requestTimeout = previous })
	account := func(label string) Row {
		return Row{"display_name": "Slow " + label, "email": "slow-" + label + "-" + newID() + "@example.test", "password": "Slow derivation 8!"}
	}
	user := account("user")
	platform.want("POST", "/admin/v1/users", user, 0, 201)
	tenant := newID()
	platform.want("POST", "/admin/v1/tenants", Row{"id": tenant, "slug": "slow-" + tenant, "display_name": "Slow derivation", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin": account("admin")}, 0, 201)
	platform.want("POST", "/v1/tenants/"+tenant+"/memberships", Row{"account": account("member")}, 0, 201)
	platform.want("POST", "/admin/v1/tenants/"+tenant+"/administrators", Row{"account": account("administrator")}, 0, 201)
	// A password change derives twice (current and new password).
	signedIn := passwordSignIn(platform, str(user["email"]), "Slow derivation 8!", 200)
	signedIn.want("POST", "/auth/password", Row{"current_password": "Slow derivation 8!", "new_password": "Fast derivation 9!"}, 0, 200)
}

func passwordSignIn(c testClient, email, password string, want int) testClient {
	c.t.Helper()
	b, _ := json.Marshal(Row{"email": email, "password": password})
	r := httptest.NewRequest("POST", "/auth/login/password", bytes.NewReader(b))
	r.Header.Set("Origin", c.s.Config.PublicURL)
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != want {
		c.t.Fatalf("password login: got %d want %d", w.Code, want)
	}
	if want == 200 {
		for _, cookie := range w.Result().Cookies() {
			if cookie.Name == c.s.cookieName("lastapp_session") {
				c.session = cookie.Value
			}
			if cookie.Name == c.s.cookieName("lastapp_csrf") {
				c.csrf = cookie.Value
			}
		}
	}
	return c
}

func TestLocalBootstrapAndLoginGuards(t *testing.T) {
	s, c, _, _, _ := foundationFixture(t)
	s.Config.OIDCIssuer = ""
	if e := s.InitAuth(context.Background()); e != nil {
		t.Fatal(e)
	}
	email, password := "local-"+newID()+"@example.test", "Boot8!ab"
	user, e := s.BootstrapLocal(context.Background(), "Local operator", email, password)
	if e != nil {
		t.Fatal(e)
	}
	admin := passwordSignIn(c, email, password, 200)
	me := admin.want("GET", "/v1/me", nil, 0, 200)
	if me["platform_admin"] != true || me["password_change_required"] != true {
		t.Fatal("local bootstrap authority", me)
	}
	if _, e = s.BootstrapLocal(context.Background(), "Local operator", email, ""); e != nil {
		t.Fatal("idempotent local bootstrap", e)
	}
	if _, e = s.DB.Exec("UPDATE local_accounts SET password_iterations=1 WHERE user_id=?", user); e == nil {
		t.Fatal("unsafe password cost accepted")
	}
	for _, origin := range []string{"", "https://untrusted.example"} {
		b, _ := json.Marshal(Row{"email": email, "password": password})
		r := httptest.NewRequest("POST", "/auth/login/password", bytes.NewReader(b))
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		if w.Code != 403 || len(w.Result().Cookies()) != 0 {
			t.Fatal("login origin guard failed", w.Code)
		}
	}
	// Exhaust just the email budget at a fixed timestamp, without expensive hashes.
	for range 8 {
		s.loginLimiter.allowRate("email:"+hex.EncodeToString(hash("limited@example.test")), time.Now(), 8, 1.0/30)
	}
	passwordSignIn(c, "limited@example.test", password, 429)
	wrong := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, wrong)
	if w.Code != 404 {
		t.Fatal("SSO route available without a provider", w.Code)
	}
}

func TestTenantAccountsAndPlatformSeparation(t *testing.T) {
	s, platform, operator, oidcOwner, _ := foundationFixture(t)
	tenant, email := newID(), "owner-"+newID()+"@example.test"
	temporary, personal := "Temp8!ab", "Own8!xyz"
	create := Row{"id": tenant, "slug": "accounts-" + tenant, "display_name": "Account workspace", "plan_id": "10000000-0000-4000-8000-000000000001", "initial_admin": Row{"display_name": "Account owner", "email": email, "password": temporary}}
	platform.want("POST", "/admin/v1/tenants", create, 0, 201)
	base := "/v1/tenants/" + tenant
	members := platform.want("GET", base+"/memberships", nil, 0, 200)["items"].([]any)
	user := str(members[0].(map[string]any)["user_id"])
	passwordSignIn(platform, email, "Wrong password", 401)
	passwordSignIn(platform, "unknown@example.test", temporary, 401)
	admin := passwordSignIn(platform, email, temporary, 200)
	me := admin.want("GET", "/v1/me", nil, 0, 200)
	if me["password_change_required"] != true || me["platform_admin"] != false {
		t.Fatal("initial local account has wrong authority", me)
	}
	admin.want("GET", base+"/overview", nil, 0, 403)
	admin.want("POST", base+"/spaces", Row{}, 0, 403)
	oldSession := passwordSignIn(platform, email, temporary, 200)
	admin.want("POST", "/auth/password", Row{"current_password": "wrong", "new_password": personal}, 0, 401)
	// The successful password change rotates the current session and revokes all others.
	b, _ := json.Marshal(Row{"current_password": temporary, "new_password": personal})
	r := httptest.NewRequest("POST", "/auth/password", bytes.NewReader(b))
	r.Header.Set("Origin", s.Config.PublicURL)
	r.Header.Set("X-CSRF-Token", admin.csrf)
	r.AddCookie(&http.Cookie{Name: s.cookieName("lastapp_session"), Value: admin.session})
	w := httptest.NewRecorder()
	admin.h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("password change failed: %d", w.Code)
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == s.cookieName("lastapp_session") {
			admin.session = cookie.Value
		}
		if cookie.Name == s.cookieName("lastapp_csrf") {
			admin.csrf = cookie.Value
		}
	}
	oldSession.want("GET", "/v1/me", nil, 0, 401)
	if admin.want("GET", "/v1/me", nil, 0, 200)["password_change_required"] != false {
		t.Fatal("password change requirement persisted")
	}
	passwordSignIn(platform, email, temporary, 401)
	passwordSignIn(platform, email, personal, 200)
	admin.want("GET", base+"/overview", nil, 0, 200)
	admin.want("GET", "/admin/v1/users", nil, 0, 403)
	platform.want("POST", base+"/memberships", Row{"user_id": operator}, 0, 201)
	platform.want("POST", base+"/role-assignments", Row{"id": newID(), "user_id": operator, "role_id": TenantAdminRole}, 0, 400)
	platform.want("POST", "/admin/v1/role-assignments", Row{"id": newID(), "user_id": user, "role_id": SuperRole}, 0, 409)
	// Creation of another Tenant never replaces this global user's password.
	another := newID()
	create["id"], create["slug"] = another, "existing-"+another
	platform.want("POST", "/admin/v1/tenants", create, 0, 409)
	create["initial_admin"] = Row{"email": email}
	platform.want("POST", "/admin/v1/tenants", create, 0, 201)
	var sameUser string
	if e := s.DB.QueryRow("SELECT user_id FROM tenant_memberships WHERE tenant_id=?", another).Scan(&sameUser); e != nil || sameUser != user {
		t.Fatal("existing account was not reused", e)
	}
	// Fixture metadata is enough to exercise resource authority without provisioning a new server.
	target, db, space, app := newID(), newID(), newID(), newID()
	platform.want("POST", "/admin/v1/database-targets", Row{"id": target, "display_name": "Account fixture", "canonical_host": "127.0.0.1", "port": 33317, "tls_mode": "disabled"}, 0, 201)
	enc, e := s.seal("database", db, tenant, credentials{Username: "fixture", Password: token(32)})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO database_resources(id,tenant_id,target_id,database_name,management_mode,layout,state,runtime_credentials_enc) VALUES (?,?,?,?,'registered','shared','ready',?)", db, tenant, target, "fixture_"+db, enc); e != nil {
		t.Fatal(e)
	}
	body := Row{"id": space, "slug": "sales", "display_name": "Sales", "initial_manager_user_id": user, "database": Row{"resource_id": db}}
	platform.want("POST", base+"/spaces", body, 0, 403)
	admin.want("POST", base+"/spaces", body, 0, 201)
	body = Row{"id": app, "slug": "crm", "display_name": "CRM", "initial_manager_user_id": user}
	platform.want("POST", base+"/spaces/"+space+"/apps", body, 0, 403)
	admin.want("POST", base+"/spaces/"+space+"/apps", body, 0, 201)
	platform.want("PATCH", base+"/apps/"+app, Row{"display_name": "Forbidden"}, 1, 403)
	platform.want("POST", base+"/apps/"+app+"/microapps", Row{"id": newID(), "slug": "forbidden", "display_name": "Forbidden"}, 0, 403)
	platform.want("POST", base+"/spaces/"+space+"/roles", Row{"id": newID(), "code": "forbidden", "display_name": "Forbidden"}, 0, 403)
	platform.want("POST", base+"/teams", Row{"id": newID(), "slug": "forbidden", "display_name": "Forbidden"}, 0, 403)
	admin.want("POST", base+"/apps/"+app+"/microapps", Row{"id": newID(), "slug": "deals", "display_name": "Deals"}, 0, 201)
	admin.want("PATCH", base, Row{"display_name": "My company"}, 1, 200)
	admin.want("PATCH", base, Row{"plan_id": "10000000-0000-4000-8000-000000000002"}, 2, 400)
	// Account creation/membership is atomic and credential data never enters audit.
	memberEmail := "member-" + newID() + "@example.test"
	memberAccount := Row{"display_name": "New member", "email": memberEmail, "password": temporary}
	admin.want("POST", base+"/memberships", Row{"account": memberAccount}, 0, 403)
	admin.want("POST", "/admin/v1/users", memberAccount, 0, 403)
	var n int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM local_accounts WHERE login_email=?", memberEmail).Scan(&n); e != nil || n != 0 {
		t.Fatal("Tenant administrator claimed a global identity", e)
	}
	// Only the trusted platform provisions global credentials. Tenant administrators
	// can reuse that identity without knowing or replacing its password.
	member := platform.want("POST", "/admin/v1/users", memberAccount, 0, 201)
	platform.want("POST", "/admin/v1/users", memberAccount, 0, 409)
	admin.want("POST", base+"/memberships", Row{"account": Row{"email": memberEmail}}, 0, 201)
	admin.want("POST", "/v1/tenants/"+another+"/memberships", Row{"account": memberAccount}, 0, 403)
	added := admin.want("POST", "/v1/tenants/"+another+"/memberships", Row{"account": Row{"email": memberEmail}}, 0, 201)
	if added["user_id"] != member["id"] {
		t.Fatal("trusted global account not reused")
	}
	passwordSignIn(platform, memberEmail, temporary, 200)
	if strings.Contains(str(member), temporary) || strings.Contains(str(member), "password_hash") {
		t.Fatal("user creation exposed credentials")
	}
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE CAST(details_json AS CHAR) LIKE ?", "%"+temporary+"%").Scan(&n); e != nil || n != 0 {
		t.Fatal("password entered audit history", e)
	}
	platform.as(oidcOwner).want("POST", "/auth/password", Row{"current_password": temporary, "new_password": personal}, 0, 400)
	badID := newID()
	create["id"], create["slug"], create["plan_id"], create["initial_admin"] = badID, "rollback-"+badID, newID(), Row{"display_name": "Rollback", "email": "rollback-" + badID + "@example.test", "password": temporary}
	platform.want("POST", "/admin/v1/tenants", create, 0, 404)
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM local_accounts WHERE login_email=?", "rollback-"+badID+"@example.test").Scan(&n); e != nil || n != 0 {
		t.Fatal("failed Tenant create left an account", e)
	}
}

// TestAccountDeadlineRenewsOnlyAfterDerivation pins that only a derived
// password renews the request deadline; choosing an existing account keeps it.
func TestAccountDeadlineRenewsOnlyAfterDerivation(t *testing.T) {
	ctx, cancel := withRequestDeadline(context.Background())
	defer cancel()
	original, _ := ctx.Deadline()
	time.Sleep(10 * time.Millisecond)
	kept, keep := preparedAccount{Email: "existing@example.test"}.deadline(ctx)
	defer keep()
	if deadline, _ := kept.Deadline(); !deadline.Equal(original) {
		t.Fatal("an account without a derived password renewed the request deadline")
	}
	renewed, stop := preparedAccount{Email: "new@example.test", Hash: []byte{1}, Salt: []byte{2}}.deadline(ctx)
	defer stop()
	if deadline, _ := renewed.Deadline(); !deadline.After(original) {
		t.Fatal("a derived password did not renew the request deadline")
	}
}
