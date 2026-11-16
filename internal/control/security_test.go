package control

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/registry"
)

// Regression tests for the ten independently reproduced security review findings.
// They run against a disposable registry and remove only their own databases/users.
func TestSecurityRegressions(t *testing.T) {
	configPath := os.Getenv("LASTAPP_SECURITY_CONFIG")
	if configPath == "" {
		t.Skip("set LASTAPP_SECURITY_CONFIG to local .local/config.json")
	}
	source, e := LoadConfig(configPath)
	if e != nil {
		t.Fatal(e)
	}
	if !source.Development {
		t.Fatal("review requires local development configuration")
	}
	adminCfg, e := LoadConfig(filepath.Join(filepath.Dir(configPath), "migrate.json"))
	if e != nil {
		t.Fatal(e)
	}
	admin, e := Open(adminCfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { admin.DB.Close() })
	reviewRegistry := "lastapp_review_" + strings.ReplaceAll(registry.NewID(), "-", "")
	if _, e = admin.DB.Exec("CREATE DATABASE `" + reviewRegistry + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); e != nil {
		t.Fatal(e)
	}
	cfg := source
	slash := strings.LastIndex(adminCfg.RegistryDSN, "/")
	cfg.RegistryDSN = adminCfg.RegistryDSN[:slash+1] + reviewRegistry
	s, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	var cancel context.CancelFunc
	var done chan struct{}
	t.Cleanup(func() {
		if cancel != nil {
			cancel()
			<-done
		}
		resources, _ := registry.QueryRows(context.Background(), s.DB, "SELECT id,database_name,LOWER(HEX(ownership_token)) AS ownership_hex FROM database_resources WHERE management_mode='managed'")
		for _, r := range resources {
			name := registry.Str(r["database_name"])
			var comment string
			if err := admin.DB.QueryRow("SELECT SCHEMA_COMMENT FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", name).Scan(&comment); err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Error("cleanup lookup failed", err)
				}
				continue
			}
			if generatedRE.MatchString(name) && comment == "lastapp:"+registry.Str(r["id"])+":"+registry.Str(r["ownership_hex"]) {
				if _, err := admin.DB.Exec("DROP DATABASE `" + name + "`"); err != nil {
					t.Error("review database cleanup failed", err)
				}
				if _, err := admin.DB.Exec("DROP USER IF EXISTS '" + name + "'@'" + cfg.RuntimeAccountHost + "'"); err != nil {
					t.Error("review account cleanup failed", err)
				}
			}
		}
		s.DB.Close()
		if _, err := admin.DB.Exec("DROP DATABASE `" + reviewRegistry + "`"); err != nil {
			t.Error("review registry cleanup failed", err)
		}
	})
	if e = Migrate(context.Background(), s.DB); e != nil {
		t.Fatal(e)
	}
	var state struct {
		Provisioner string `json:"provisioner_password"`
	}
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(configPath), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &state); e != nil {
		t.Fatal(e)
	}
	if e = s.SeedDevelopment(context.Background(), state.Provisioner); e != nil {
		t.Fatal(e)
	}
	h := s.Handler()
	var operator, owner, outsideUser string
	if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='operator'").Scan(&operator); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='tenant-admin'").Scan(&owner); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.QueryRow("SELECT id FROM users WHERE oidc_subject='outsider'").Scan(&outsideUser); e != nil {
		t.Fatal(e)
	}
	c := (testClient{t: t, s: s, h: h}).as(operator)
	ownerClient := c.as(owner)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done = make(chan struct{})
	go func() { defer close(done); s.RunWorker(ctx) }()

	createTenant := func(plan string) string {
		tenant := registry.NewID()
		c.want("POST", "/admin/v1/tenants", registry.Row{"id": tenant, "slug": "review-" + tenant, "display_name": "Disposable review Tenant", "plan_id": plan, "initial_admin_user_id": owner}, 0, 201)
		return tenant
	}
	starter := "10000000-0000-4000-8000-000000000002"
	development := "10000000-0000-4000-8000-000000000001"
	target := "20000000-0000-4000-8000-000000000001"
	t.Run("FreshSchemaPreservesExactIdentityAndRejectsDuplicateGrants", func(t *testing.T) {
		user, e := s.Bootstrap(context.Background(), "https://foundation.example", "immutable-subject", "Original", "")
		if e != nil {
			t.Fatal(e)
		}
		_, e = s.DB.Exec("INSERT INTO access_grants(id,scope_kind,domain,user_id,role_id,role_owner_key) VALUES (?,'platform','control',?,?,'builtin')", registry.NewID(), user, access.SuperRole)
		if e == nil {
			t.Fatal("duplicate grant accepted")
		}
		if e = Migrate(context.Background(), s.DB); e != nil {
			t.Fatal(e)
		}
		var found string
		if e = s.DB.QueryRow("SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject=?", "https://foundation.example", "immutable-subject").Scan(&found); e != nil || found != user {
			t.Fatal("identity changed", e)
		}
		if e = CheckSchema(context.Background(), s.DB); e != nil {
			t.Fatal(e)
		}
	})
	businessScope := func(tenant string) string {
		space := registry.NewID()
		ownerClient.want("POST", "/v1/tenants/"+tenant+"/spaces", registry.Row{"id": space, "initial_manager_user_id": owner, "slug": "scope-" + space, "display_name": "Role fixture", "database": registry.Row{"mode": "managed", "layout": "shared", "target_id": target}}, 0, 201)
		return "/v1/tenants/" + tenant + "/spaces/" + space
	}
	t.Run("MetadataCeilingsAndBoundedGroupMembers", func(t *testing.T) {
		tenant := createTenant(development)
		base := "/v1/tenants/" + tenant
		group := registry.NewID()
		ownerClient.want("POST", base+"/teams", registry.Row{"id": group, "slug": "bounded", "display_name": "Bounded"}, 0, 201)
		for i := 0; i < 12; i++ {
			user := registry.NewID()
			if _, e = s.DB.Exec("INSERT INTO users (id,oidc_issuer,oidc_subject,display_name) VALUES (?,?,?,?)", user, cfg.OIDCIssuer, user, fmt.Sprintf("Member %02d", i)); e != nil {
				t.Fatal(e)
			}
			if _, e = s.DB.Exec("INSERT INTO tenant_memberships (tenant_id,user_id) VALUES (?,?)", tenant, user); e != nil {
				t.Fatal(e)
			}
			if _, e = s.DB.Exec("INSERT INTO group_memberships (tenant_id,group_id,user_id) VALUES (?,?,?)", tenant, group, user); e != nil {
				t.Fatal(e)
			}
		}
		first := ownerClient.want("GET", base+"/teams/"+group+"/members?limit=10", nil, 0, 200)
		if len(first["items"].([]any)) != 10 || first["pagination"].(map[string]any)["has_more"] != true {
			t.Fatal("unbounded team members", first)
		}
		later := ownerClient.want("GET", base+"/teams/"+group+"/members?page=2&limit=10", nil, 0, 200)["items"].([]any)
		if len(later) != 2 {
			t.Fatal("group member pagination incomplete", later)
		}
		for i := 1; i < registry.MetadataLimits["groups"]; i++ {
			if _, e = s.DB.Exec("INSERT INTO groups (id,tenant_id,slug,display_name) VALUES (?,?,?,?)", registry.NewID(), tenant, fmt.Sprintf("group-%03d", i), "Bounded"); e != nil {
				t.Fatal(e)
			}
		}
		ownerClient.want("POST", base+"/teams", registry.Row{"id": registry.NewID(), "slug": "over-limit", "display_name": "Denied"}, 0, 409)
	})
	t.Run("SecurityRoleAuditHasBeforeAndAfter", func(t *testing.T) {
		tenant := createTenant(development)
		base := businessScope(tenant)
		role := registry.NewID()
		ownerClient.want("POST", base+"/roles", registry.Row{"id": role, "code": "audited", "display_name": "Audited"}, 0, 201)
		ownerClient.want("PATCH", base+"/roles/"+role, registry.Row{"state": "disabled"}, 1, 200)
		var raw string
		if e = s.DB.QueryRow("SELECT details_json FROM audit_events WHERE resource_id=? AND action='role.updated'", role).Scan(&raw); e != nil {
			t.Fatal(e)
		}
		var details struct {
			Before, After struct {
				State       string
				Permissions []string
			}
		}
		if e = json.Unmarshal([]byte(raw), &details); e != nil || details.Before.State != "active" || details.After.State != "disabled" || len(details.Before.Permissions) != 0 || len(details.After.Permissions) != 0 {
			t.Fatalf("role audit incomplete: %s", raw)
		}
	})
	t.Run("ExpiredSessionsAreRemovedWithoutRevokingLiveSessions", func(t *testing.T) {
		expired, live := registry.Token(32), registry.Token(32)
		if _, e = s.DB.Exec("INSERT INTO sessions (token_hash,user_id,csrf_hash,created_at,expires_at) VALUES (?,?,?,UTC_TIMESTAMP(6)-INTERVAL 2 HOUR,UTC_TIMESTAMP(6)-INTERVAL 1 HOUR)", hash(expired), operator, hash(registry.Token(32))); e != nil {
			t.Fatal(e)
		}
		if _, e = s.DB.Exec("INSERT INTO sessions (token_hash,user_id,csrf_hash,expires_at) VALUES (?,?,?,UTC_TIMESTAMP(6)+INTERVAL 1 HOUR)", hash(live), operator, hash(registry.Token(32))); e != nil {
			t.Fatal(e)
		}
		if _, e = cleanupSessions(context.Background(), s.DB); e != nil {
			t.Fatal(e)
		}
		var count int
		if e = s.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE token_hash=?", hash(expired)).Scan(&count); e != nil || count != 0 {
			t.Fatal("expired session retained", e)
		}
		if e = s.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE token_hash=?", hash(live)).Scan(&count); e != nil || count != 1 {
			t.Fatal("live session revoked", e)
		}
	})
	t.Run("QueueFairness", func(t *testing.T) {
		a, b := createTenant(development), createTenant(development)
		if a > b {
			a, b = b, a
		}
		// Paused worker eligibility is controlled by suspending these Tenants until
		// both rows exist; selection itself runs in a transaction and rolls back.
		tx, e := s.DB.BeginTx(context.Background(), nil)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		for _, tenant := range []string{a, b} {
			for i := 0; i < 2; i++ {
				record := registry.NewID()
				if _, e = tx.Exec("INSERT INTO database_resources (id,tenant_id,target_id,database_name,management_mode,layout,runtime_credentials_enc,ownership_token) VALUES (?,?,?,?,'managed','shared','test',RANDOM_BYTES(16))", record, tenant, target, "la_"+strings.ReplaceAll(record, "-", "")); e != nil {
					t.Fatal(e)
				}
			}
		}
		first, e := nextJob(context.Background(), tx, "")
		if e != nil {
			t.Fatal(e)
		}
		next, e := nextJob(context.Background(), tx, first.TenantID)
		if e != nil || next.TenantID == first.TenantID {
			t.Fatal("worker did not yield to another Tenant", e)
		}
	})
	t.Run("ExactOIDCSubjectsRemainSeparate", func(t *testing.T) {
		key, _ := rsa.GenerateKey(rand.Reader, 2048)
		issuer, nonce := "", ""
		encode := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
		provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/.well-known/openid-configuration":
				if err := json.NewEncoder(w).Encode(registry.Row{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}}); err != nil {
					t.Error(err)
				}
			case "/jwks":
				if err := json.NewEncoder(w).Encode(registry.Row{"keys": []any{registry.Row{"kty": "RSA", "kid": "review", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}}); err != nil {
					t.Error(err)
				}
			case "/token":
				unsigned := encode(registry.Row{"alg": "RS256", "kid": "review"}) + "." + encode(registry.Row{"iss": issuer, "sub": "review-victim ", "aud": cfg.OIDCClientID, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(), "nonce": nonce, "name": "Distinct provider account"})
				digest := sha256.Sum256([]byte(unsigned))
				signature, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
				if err := json.NewEncoder(w).Encode(registry.Row{"access_token": "review-access-token", "token_type": "Bearer", "expires_in": 60, "id_token": unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)}); err != nil {
					t.Error(err)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer provider.Close()
		issuer = provider.URL
		s.Config.OIDCIssuer = issuer
		if e = s.InitAuth(context.Background()); e != nil {
			t.Fatal(e)
		}
		victim, e := s.Bootstrap(context.Background(), issuer, "review-victim", "Original administrator", "")
		if e != nil {
			t.Fatal(e)
		}
		start := httptest.NewRecorder()
		h.ServeHTTP(start, httptest.NewRequest("GET", "/auth/login", nil))
		dest, _ := url.Parse(start.Header().Get("Location"))
		nonce = dest.Query().Get("nonce")
		request := httptest.NewRequest("GET", "/auth/callback?code=review&state="+url.QueryEscape(dest.Query().Get("state")), nil)
		for _, cookie := range start.Result().Cookies() {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		if response.Code != 302 {
			t.Fatalf("callback: %d %s", response.Code, response.Body.String())
		}
		meReq := httptest.NewRequest("GET", "/v1/me", nil)
		for _, cookie := range response.Result().Cookies() {
			meReq.AddCookie(cookie)
		}
		me := httptest.NewRecorder()
		h.ServeHTTP(me, meReq)
		var identity registry.Row
		if err := json.Unmarshal(me.Body.Bytes(), &identity); err != nil {
			t.Fatal(err)
		}
		if registry.Str(identity["id"]) == victim || identity["platform_admin"] != false {
			t.Fatalf("distinct signed identity inherited another account: %v", identity)
		}
	})
	t.Run("RetainedDatabasesConsumeQuota", func(t *testing.T) {
		tenant := createTenant(starter)
		base := "/v1/tenants/" + tenant
		tc := c.as(owner)
		ids := []string{registry.NewID(), registry.NewID()}
		tc.want("POST", base+"/database-resources", registry.Row{"id": ids[0], "management_mode": "managed", "layout": "shared", "target_id": target}, 0, 202)
		ready := tc.waitDB(tenant, ids[0], "ready")
		tc.want("PATCH", base+"/database-resources/"+ids[0], registry.Row{"state": "retired"}, registry.Integer(ready["revision"]), 202)
		tc.want("POST", base+"/database-resources", registry.Row{"id": ids[1], "management_mode": "managed", "layout": "shared", "target_id": target}, 0, 409)
		var physical int
		if e = admin.DB.QueryRow("SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME IN (?,?)", "la_"+strings.ReplaceAll(ids[0], "-", ""), "la_"+strings.ReplaceAll(ids[1], "-", "")).Scan(&physical); e != nil || physical != 1 {
			t.Fatalf("retirement released physical quota: count=%d error=%v", physical, e)
		}
	})
	t.Run("ProductionRegistryRejectsPlaintextTCP", func(t *testing.T) {
		production := source
		production.Development = false
		production.PublicURL = "https://review.example"
		production.Address = "127.0.0.1:0"
		opened, e := Open(production)
		if e == nil {
			opened.DB.Close()
			t.Fatal("production accepted plaintext registry TCP")
		}
	})
	t.Run("GlobalGrantOptionRejected", func(t *testing.T) {
		name := "rv_" + strings.ReplaceAll(registry.NewID(), "-", "")
		password := registry.Token(32)
		if _, e = admin.DB.Exec("CREATE DATABASE `" + name + "`"); e != nil {
			t.Fatal(e)
		}
		if _, e = admin.DB.Exec("CREATE USER '" + name + "'@'" + cfg.RuntimeAccountHost + "' IDENTIFIED BY '" + password + "'"); e != nil {
			t.Fatal(e)
		}
		defer func() {
			if _, err := admin.DB.Exec("DROP DATABASE `" + name + "`"); err != nil {
				t.Error("review database cleanup failed", err)
			}
		}()
		defer func() {
			if _, err := admin.DB.Exec("DROP USER '" + name + "'@'" + cfg.RuntimeAccountHost + "'"); err != nil {
				t.Error("review account cleanup failed", err)
			}
		}()
		if _, e = admin.DB.Exec("GRANT USAGE ON *.* TO '" + name + "'@'" + cfg.RuntimeAccountHost + "' WITH GRANT OPTION"); e != nil {
			t.Fatal(e)
		}
		if _, e = admin.DB.Exec("GRANT SELECT,INSERT,UPDATE,DELETE ON `" + strings.ReplaceAll(name, "_", `\_`) + "`.* TO '" + name + "'@'" + cfg.RuntimeAccountHost + "'"); e != nil {
			t.Fatal(e)
		}
		targetRow, _ := targetEndpoint(context.Background(), s.DB, target)
		db, e := s.targetDB(context.Background(), targetRow, credentials{Username: name, Password: password}, name)
		if e != nil {
			t.Fatal(e)
		}
		defer db.Close()
		if e = validateRuntime(context.Background(), db, name, true); e == nil {
			t.Fatal("global GRANT OPTION accepted")
		}
		if e = validateRuntime(context.Background(), db, name, false); e == nil {
			t.Fatal("registered account grant authority accepted")
		}
		if _, e = admin.DB.Exec("REVOKE GRANT OPTION ON *.* FROM '" + name + "'@'" + cfg.RuntimeAccountHost + "'"); e != nil {
			t.Fatal(e)
		}
		if e = validateRuntime(context.Background(), db, name, true); e != nil {
			t.Fatal("safe CRUD rejected", e)
		}
	})
	t.Run("GrantAuditRetainsPrincipalAndScope", func(t *testing.T) {
		tenant := createTenant(development)
		base := "/v1/tenants/" + tenant
		ownerClient.want("POST", base+"/memberships", registry.Row{"user_id": outsideUser}, 0, 201)
		grant := registry.NewID()
		ownerClient.want("POST", base+"/role-assignments", registry.Row{"id": grant, "user_id": outsideUser, "role_id": access.TenantAdminRole}, 0, 201)
		ownerClient.want("DELETE", base+"/role-assignments/"+grant, nil, 1, 200)
		var details string
		if e = s.DB.QueryRow("SELECT details_json FROM audit_events WHERE resource_id=? AND action='grant.deleted'", grant).Scan(&details); e != nil {
			t.Fatal(e)
		}
		var change struct {
			Before struct {
				User string `json:"user_id"`
				Role string `json:"role_id"`
			}
			After any
		}
		if e = json.Unmarshal([]byte(details), &change); e != nil || change.Before.User != outsideUser || change.Before.Role != access.TenantAdminRole || change.After != nil {
			t.Fatalf("incomplete revocation audit: %s", details)
		}
	})
	t.Run("IndependentTenantsDoNotLockSharedPlans", func(t *testing.T) {
		first, second := createTenant(development), createTenant(development)
		tx, e := s.DB.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		if _, e = registry.PolicyFor(context.Background(), tx, first); e != nil {
			t.Fatal(e)
		}
		other, e := s.DB.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			t.Fatal(e)
		}
		defer other.Rollback()
		short, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		_, e = registry.PolicyFor(short, other, second)
		if e != nil {
			t.Fatal("unrelated Tenant allocation blocked", e)
		}
	})
	t.Run("BoundedCollectionsAndUniqueGrants", func(t *testing.T) {
		tenant := createTenant(development)
		base := "/v1/tenants/" + tenant
		ownerClient.want("POST", base+"/memberships", registry.Row{"user_id": outsideUser}, 0, 201)
		ownerClient.want("POST", base+"/role-assignments", registry.Row{"id": registry.NewID(), "user_id": outsideUser, "role_id": access.TenantAdminRole}, 0, 201)
		ownerClient.want("POST", base+"/role-assignments", registry.Row{"id": registry.NewID(), "user_id": outsideUser, "role_id": access.TenantAdminRole}, 0, 409)
		scope := businessScope(tenant)
		space := scope[strings.LastIndex(scope, "/")+1:]
		for i := 0; i < 105; i++ {
			if _, e = s.DB.Exec("INSERT INTO roles(id,tenant_id,space_id,owner_key,scope_kind,domain,code,display_name) VALUES (?,?,?,?,'space','data',?,?)", registry.NewID(), tenant, space, "s:"+space, fmt.Sprintf("paged-%03d", i), fmt.Sprintf("Paged %03d", i)); e != nil {
				t.Fatal(e)
			}
		}
		first := c.want("GET", scope+"/roles", nil, 0, 200)
		second := c.want("GET", scope+"/roles?page=2", nil, 0, 200)
		if len(first["items"].([]any)) != 100 || first["pagination"].(map[string]any)["has_more"] != true || len(second["items"].([]any)) != 6 {
			t.Fatal("unbounded/incomplete role pagination")
		}
		if len(c.want("GET", scope+"/roles?q=Paged%20104", nil, 0, 200)["items"].([]any)) != 1 {
			t.Fatal("search did not reach later pages")
		}
	})
	t.Run("DedicatedAppGrandfathering", func(t *testing.T) {
		tenant := createTenant(development)
		base := "/v1/tenants/" + tenant
		space := registry.NewID()
		ownerClient.want("POST", base+"/spaces", registry.Row{"initial_manager_user_id": owner, "id": space, "slug": "review-space", "display_name": "Review", "database": registry.Row{"mode": "managed", "layout": "shared", "target_id": target}}, 0, 201)
		app := registry.NewID()
		ownerClient.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"initial_manager_user_id": owner, "id": app, "slug": "review-app", "display_name": "Dedicated app", "compute_mode": "dedicated"}, 0, 201)
		current := c.want("GET", base, nil, 0, 200)
		c.want("PATCH", "/admin/v1/tenants/"+tenant, registry.Row{"plan_id": starter}, registry.Integer(current["revision"]), 200)
		ownerClient.want("PATCH", base+"/apps/"+app, registry.Row{"display_name": "Rename", "compute_mode": "dedicated"}, 1, 200)
		ownerClient.want("PATCH", base+"/apps/"+app, registry.Row{"display_name": "Rename again"}, 2, 200)
		ownerClient.want("PATCH", base+"/apps/"+app, registry.Row{"compute_mode": "shared"}, 3, 200)
		ownerClient.want("PATCH", base+"/apps/"+app, registry.Row{"compute_mode": "dedicated"}, 4, 403)
	})
	t.Run("DisabledRoleCreation", func(t *testing.T) {
		tenant := createTenant(development)
		base := businessScope(tenant)
		role := registry.NewID()
		ownerClient.want("POST", base+"/roles", registry.Row{"id": role, "code": "disabled-role", "display_name": "Requested disabled", "state": "disabled"}, 0, 201)
		var state string
		if e = s.DB.QueryRow("SELECT state FROM roles WHERE id=?", role).Scan(&state); e != nil {
			t.Fatal(e)
		}
		if state != "disabled" {
			t.Fatalf("disabled role became %s", state)
		}
		ownerClient.want("POST", base+"/roles", registry.Row{"id": registry.NewID(), "code": "invalid-state", "display_name": "Invalid", "state": "bogus"}, 0, 400)
	})
	t.Run("SuspensionBlocksAllNewInventory", func(t *testing.T) {
		tenant := createTenant(development)
		base := "/v1/tenants/" + tenant
		space, app := registry.NewID(), registry.NewID()
		ownerClient.want("POST", base+"/spaces", registry.Row{"initial_manager_user_id": owner, "id": space, "slug": "lifecycle", "display_name": "Lifecycle", "database": registry.Row{"mode": "managed", "layout": "shared", "target_id": target}}, 0, 201)
		ownerClient.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"initial_manager_user_id": owner, "id": app, "slug": "lifecycle", "display_name": "Lifecycle"}, 0, 201)
		current := c.want("GET", base, nil, 0, 200)
		c.want("PATCH", "/admin/v1/tenants/"+tenant, registry.Row{"state": "suspended"}, registry.Integer(current["revision"]), 200)
		ownerClient.want("POST", base+"/apps/"+app+"/microapps", registry.Row{"id": registry.NewID(), "slug": "still-created", "display_name": "Still created"}, 0, 403)
		ownerClient.want("POST", base+"/spaces/"+space+"/apps", registry.Row{"initial_manager_user_id": owner, "id": registry.NewID(), "slug": "blocked", "display_name": "Blocked"}, 0, 403)
	})

	fmt.Println("Review fixture registry and all its managed databases/accounts are removed by test cleanup.")
}
