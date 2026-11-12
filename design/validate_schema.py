#!/usr/bin/env python3
"""Validate the design DDL in a private, temporary, Unix-socket-only MariaDB.

Uses MariaDB command-line binaries and Python's standard library. It never
connects to a pre-existing server. This verifies SQL constraints, not Go APIs.
"""
import datetime
import pathlib
import os
import selectors
import shutil
import subprocess
import tempfile
import time
import uuid

HERE = pathlib.Path(__file__).resolve().parent
checks = []


def uid(number):
    return str(uuid.UUID(int=number))


def quote(value):
    return "'" + str(value).replace("'", "''") + "'"


def main():
    client = shutil.which("mariadb")
    daemon = shutil.which("mariadbd")
    installer = shutil.which("mariadb-install-db")
    if not all((client, daemon, installer)):
        raise RuntimeError("mariadb, mariadbd and mariadb-install-db must be installed")
    basedir = str(pathlib.Path(daemon).resolve().parent.parent)
    server = None
    processes = []
    version = "server not started"
    failure = None
    with tempfile.TemporaryDirectory(prefix="lastapp-schema-") as temporary:
        root = pathlib.Path(temporary)
        data = root / "data"
        socket = root / "m.sock"
        try:
            initialized = subprocess.run(
                [installer, "--no-defaults", "--basedir=" + basedir,
                 "--datadir=" + str(data), "--auth-root-authentication-method=normal",
                 "--skip-test-db"],
                capture_output=True, text=True, timeout=60,
            )
            if initialized.returncode:
                raise RuntimeError(initialized.stdout[-2000:] + initialized.stderr[-2000:])
            server = subprocess.Popen(
                [daemon, "--no-defaults", "--basedir=" + basedir,
                 "--datadir=" + str(data), "--socket=" + str(socket),
                 "--pid-file=" + str(root / "server.pid"), "--skip-networking",
                 "--log-error=" + str(root / "server.err"),
                 "--innodb-buffer-pool-size=64M", "--innodb-log-file-size=32M",
                 "--tmpdir=" + str(root)],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            )
            base = [client, "--no-defaults", "--protocol=socket",
                    "--socket=" + str(socket), "--user=root", "--batch",
                    "--skip-column-names", "--unbuffered", "--default-character-set=utf8mb4"]

            def run(sql, expect_error=None, database=True):
                args = base + (["lastapp_schema_validation"] if database else [])
                result = subprocess.run(args, input=sql, text=True, capture_output=True, timeout=15)
                if expect_error is not None:
                    if result.returncode == 0 or ("ERROR " + str(expect_error)) not in result.stderr:
                        raise AssertionError("Expected error " + str(expect_error) + ": " + result.stderr)
                elif result.returncode:
                    raise AssertionError(result.stderr + "\nSQL: " + sql[:1000])
                return result.stdout.strip()

            for _ in range(100):
                if server.poll() is not None:
                    raise RuntimeError((root / "server.err").read_text()[-4000:])
                ready = subprocess.run(base, input="SELECT 1;", text=True, capture_output=True)
                if ready.returncode == 0:
                    break
                time.sleep(0.1)
            else:
                raise RuntimeError("Private MariaDB did not become ready")
            version = run("SELECT VERSION();", database=False)
            run("CREATE DATABASE lastapp_schema_validation CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
                database=False)
            run((HERE / "control_plane.sql").read_text())
            assert run("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE();") == "17"
            checks.append("DDL applies to an empty database and creates all 17 tables")

            def insert(table, values, error=None):
                columns = ", ".join(values)
                literals = ", ".join("NULL" if v is None else quote(v) for v in values.values())
                return run("INSERT INTO " + table + " (" + columns + ") VALUES (" + literals + ");", error)

            def rejected(label, table, values, error):
                insert(table, values, error)
                checks.append(label)

            admin, other = uid(1), uid(2)
            for user, subject in ((admin, "Admin"), (other, "admin")):
                insert("users", dict(id=user, oidc_issuer="https://issuer.example",
                                    oidc_subject=subject, display_name=subject))
            checks.append("OIDC subjects compare case-sensitively")
            rejected("Duplicate provider identity is rejected", "users",
                     dict(id=uid(3), oidc_issuer="https://issuer.example",
                          oidc_subject="Admin", display_name="Duplicate"), 1062)
            plan = dict(id=uid(10), code="flexible", display_name="Flexible",
                        allow_shared_database=1, allow_dedicated_database=1,
                        allow_managed_database=1, allow_registered_database=1,
                        allow_dedicated_compute=1, default_layout="shared")
            insert("plans", plan)
            rejected("A plan cannot default to a forbidden layout", "plans",
                     dict(plan, id=uid(11), code="bad", allow_shared_database=0), 4025)
            ta, tb = uid(20), uid(21)
            for tenant, slug in ((ta, "tenant-a"), (tb, "tenant-b")):
                insert("tenants", dict(id=tenant, slug=slug, display_name=slug,
                                       plan_id=uid(10), created_by=admin))
                insert("tenant_memberships", dict(tenant_id=tenant, user_id=admin))
            target = uid(40)
            # Non-secret placeholders: encryption itself is a future Go integration test.
            insert("database_targets", dict(id=target, display_name="Private target",
                                           canonical_host="db.example", managed_enabled=1,
                                           provisioning_credentials_enc="placeholder-admin-envelope"))
            rejected("Managed targets require provisioning credentials", "database_targets",
                     dict(id=uid(41), display_name="Bad", canonical_host="bad.example", managed_enabled=1), 4025)

            def resource(rid, tenant, name, mode, layout):
                return dict(id=rid, tenant_id=tenant, target_id=target, database_name=name,
                            management_mode=mode, layout=layout,
                            runtime_credentials_enc="placeholder-runtime-envelope",
                            ownership_token=("x" * 16 if mode == "managed" else None))

            shared_a, dedicated_a, shared_b = uid(50), uid(51), uid(52)
            insert("database_resources", resource(shared_a, ta, "a_shared", "managed", "shared"))
            insert("database_resources", resource(dedicated_a, ta, "a_dedicated", "registered", "dedicated"))
            insert("database_resources", resource(shared_b, tb, "b_shared", "registered", "shared"))
            insert("database_resources", resource(uid(53), ta, "a_dedicated_managed", "managed", "dedicated"))
            insert("database_resources", resource(uid(54), ta, "a_shared_registered", "registered", "shared"))
            checks.append("All four layout and management-method combinations are representable")
            rejected("A resource requires an existing Tenant", "database_resources",
                     resource(uid(55), uid(999), "bad_tenant", "registered", "shared"), 1452)
            rejected("Physical names differing only by case cannot be registered twice", "database_resources",
                     resource(uid(56), tb, "A_SHARED", "registered", "shared"), 1062)
            rejected("Managed resources require an ownership token", "database_resources",
                     dict(resource(uid(57), ta, "missing_token", "managed", "shared"), ownership_token=None), 4025)
            rejected("Unknown provisioning states are rejected", "database_resources",
                     dict(resource(uid(58), ta, "bad_state", "registered", "shared"), state="unknown"), 4025)

            def space(sid, tenant, slug, database, layout):
                return dict(id=sid, tenant_id=tenant, slug=slug, display_name=slug,
                            database_resource_id=database, database_layout=layout)

            sa, sa2, sad, sb = uid(60), uid(61), uid(62), uid(63)
            for row in (space(sa, ta, "a-one", shared_a, "shared"),
                        space(sa2, ta, "a-two", shared_a, "shared"),
                        space(sad, ta, "a-dedicated", dedicated_a, "dedicated"),
                        space(sb, tb, "b-one", shared_b, "shared")):
                insert("spaces", row)
            checks.append("Two Spaces can share one tenant database")
            rejected("A Space cannot bind to another tenant's database", "spaces",
                     space(uid(64), ta, "bad-tenant", shared_b, "shared"), 1452)
            rejected("A dedicated database cannot bind to a second Space", "spaces",
                     space(uid(65), ta, "bad-dedicated", dedicated_a, "dedicated"), 1062)
            rejected("A Space cannot mislabel a shared database as dedicated", "spaces",
                     space(uid(66), ta, "bad-layout", shared_a, "dedicated"), 1452)
            run("UPDATE tenants SET default_shared_database_id=" + quote(shared_a) + " WHERE id=" + quote(ta))
            checks.append("The tenant default can point to its own shared database")
            run("UPDATE tenants SET default_shared_database_id=" + quote(dedicated_a) + " WHERE id=" + quote(ta), 1452)
            checks.append("The tenant default cannot point to a dedicated database")
            run("UPDATE tenants SET default_shared_database_id=" + quote(shared_b) + " WHERE id=" + quote(ta), 1452)
            checks.append("The tenant default cannot point to another tenant's database")

            def app(aid, tenant, space_id, slug="crm"):
                return dict(id=aid, tenant_id=tenant, space_id=space_id, slug=slug, display_name=slug)

            aa, aa2 = uid(70), uid(71)
            insert("apps", app(aa, ta, sa))
            insert("apps", app(aa2, ta, sa2))
            checks.append("The same App slug is allowed in different Spaces")
            rejected("An App cannot claim another tenant's Space", "apps", app(uid(72), ta, sb), 1452)
            rejected("Compute preference rejects unknown values", "apps",
                     dict(app(uid(73), ta, sa, "bad-compute"), compute_mode="unknown"), 4025)
            rejected("A duplicate create ID cannot allocate another App", "apps",
                     app(aa, ta, sa2, "other-name"), 1062)
            rejected("An App slug cannot be duplicated within its Space", "apps", app(uid(74), ta, sa), 1062)
            for mid, aid in ((uid(80), aa), (uid(81), aa2)):
                insert("microapp_installations", dict(id=mid, tenant_id=ta, app_id=aid,
                       slug="review", display_name="Review", artifact_ref="artifact:sha256:example"))
            checks.append("The same pinned artifact can be referenced from different Apps")
            insert("microapp_installations", dict(id=uid(82), tenant_id=ta, app_id=aa,
                   slug="draft", display_name="Draft"))
            checks.append("An installation can be inventoried before an artifact exists")
            rejected("A micro-app cannot claim another tenant's App", "microapp_installations",
                     dict(id=uid(83), tenant_id=tb, app_id=aa, slug="bad", display_name="Bad"), 1452)
            changed = run("UPDATE apps SET display_name='Edited',revision=revision+1 WHERE id="
                          + quote(aa) + " AND revision=1; SELECT ROW_COUNT();")
            stale = run("UPDATE apps SET display_name='Stale',revision=revision+1 WHERE id="
                        + quote(aa) + " AND revision=1; SELECT ROW_COUNT();")
            assert changed == "1" and stale == "0"
            checks.append("A revision-conditional UPDATE rejects a stale writer")
            rejected("Sessions cannot reference an unknown user", "sessions",
                     dict(token_hash="t" * 32, user_id=uid(999), csrf_hash="c" * 32,
                          created_at="2026-01-01", expires_at="2026-01-02"), 1452)
            rejected("Session expiry must follow creation", "sessions",
                     dict(token_hash="u" * 32, user_id=admin, csrf_hash="c" * 32,
                          created_at="2026-01-02", expires_at="2026-01-01"), 4025)
            rejected("Audit details must be a JSON object", "audit_events",
                     dict(tenant_id=ta, actor_user_id=admin, action="test", resource_type="tenant",
                          resource_id=ta, request_id=uid(100), details_json="[]"), 4025)

            # Shared identity/authorization schema checks; not a Go permission evaluator.
            super_role = "00000000-0000-4000-8000-000000000001"
            tenant_admin_role = "00000000-0000-4000-8000-000000000002"
            assert run("SELECT COUNT(*) FROM roles WHERE tenant_id IS NULL AND owner_key='platform';") == "2"
            assert run("SELECT COUNT(*) FROM role_permissions WHERE role_id=" + quote(super_role)) == "1"
            assert run("SELECT COUNT(*) FROM role_permissions WHERE role_id=" + quote(tenant_admin_role)) == run(
                "SELECT COUNT(*) FROM permissions WHERE kind='tenant'")
            checks.append("Shared built-in roles are seeded with separate platform and Tenant actions")
            insert("tenant_memberships", dict(tenant_id=tb, user_id=other))
            ga, gb = uid(120), uid(121)
            for group, tenant in ((ga, ta), (gb, tb)):
                insert("groups", dict(id=group, tenant_id=tenant, slug="sales", display_name="Sales"))
            checks.append("Different Tenants can own teams with the same slug")
            insert("group_memberships", dict(tenant_id=ta, group_id=ga, user_id=admin))
            rejected("A team cannot include a user outside its Tenant membership", "group_memberships",
                     dict(tenant_id=ta, group_id=ga, user_id=other), 1452)
            rejected("A team membership cannot reference another Tenant's team", "group_memberships",
                     dict(tenant_id=ta, group_id=gb, user_id=admin), 1452)
            ra, rb = uid(130), uid(131)
            for role, tenant in ((ra, ta), (rb, tb)):
                insert("roles", dict(id=role, tenant_id=tenant, owner_key="t:" + tenant,
                                     kind="tenant", code="reviewer", display_name="Reviewer"))
            checks.append("Different Tenants can define roles with the same code in shared tables")
            rejected("A custom Tenant role cannot claim platform assignment kind", "roles",
                     dict(id=uid(132), tenant_id=ta, owner_key="t:" + ta, kind="platform",
                          code="bad", display_name="Bad"), 4025)
            rejected("A custom role cannot falsify its owner namespace", "roles",
                     dict(id=uid(133), tenant_id=ta, owner_key="t:" + tb, kind="tenant",
                          code="bad", display_name="Bad"), 4025)
            insert("role_permissions", dict(role_id=ra, kind="tenant", permission_code="app.read"))
            rejected("A Tenant role cannot include platform permissions", "role_permissions",
                     dict(role_id=ra, kind="tenant", permission_code="platform.admin"), 1452)
            rejected("Changing the permission mapping kind cannot elevate a Tenant role", "role_permissions",
                     dict(role_id=ra, kind="platform", permission_code="platform.admin"), 1452)

            def tenant_grant(gid, tenant=ta, **overrides):
                fields = dict(id=gid, tenant_id=tenant, kind="tenant", user_id=admin,
                              permission_code="space.read")
                fields.update(overrides)
                return fields

            insert("access_grants", dict(id=uid(200), tenant_id=None, kind="platform", user_id=admin,
                                         role_id=super_role, role_owner_key="platform"))
            checks.append("A global user can receive the protected platform role without a Tenant binding")
            for gid, tenant in ((uid(201), ta), (uid(202), tb)):
                insert("access_grants", tenant_grant(gid, tenant, permission_code=None,
                       role_id=tenant_admin_role, role_owner_key="platform"))
            checks.append("One shared Tenant administrator definition can be assigned in multiple Tenants")
            insert("access_grants", tenant_grant(uid(203), user_id=None, group_id=ga, space_id=sa,
                   permission_code=None, role_id=ra, role_owner_key="t:" + ta))
            checks.append("A team can receive its Tenant's custom role within one Space")
            insert("access_grants", tenant_grant(uid(204), app_id=aa, permission_code="app.read"))
            checks.append("A user can receive a direct App permission through the same grants table")
            insert("access_grants", tenant_grant(uid(205), user_id=None, group_id=ga, space_id=sa))
            checks.append("A team can receive a direct Space permission through the same grants table")
            rejected("A Tenant grant cannot include a user outside that Tenant", "access_grants",
                     tenant_grant(uid(206), user_id=other), 1452)
            rejected("A Tenant grant cannot include another Tenant's team", "access_grants",
                     tenant_grant(uid(207), user_id=None, group_id=gb), 1452)
            rejected("A Tenant grant cannot include another Tenant's Space", "access_grants",
                     tenant_grant(uid(208), space_id=sb), 1452)
            ab = uid(75)
            insert("apps", app(ab, tb, sb))
            rejected("A Tenant grant cannot include another Tenant's App", "access_grants",
                     tenant_grant(uid(209), app_id=ab), 1452)
            rejected("A custom role cannot be assigned outside its owner Tenant", "access_grants",
                     tenant_grant(uid(210), tb, permission_code=None, role_id=ra, role_owner_key="t:" + ta), 4025)
            rejected("Falsifying the role owner cannot bypass the custom-role boundary", "access_grants",
                     tenant_grant(uid(211), tb, permission_code=None, role_id=ra, role_owner_key="t:" + tb), 1452)
            rejected("A platform role cannot be assigned as a Tenant role", "access_grants",
                     tenant_grant(uid(212), permission_code=None, role_id=super_role, role_owner_key="platform"), 1452)
            rejected("A direct Tenant grant cannot contain a platform action", "access_grants",
                     tenant_grant(uid(213), permission_code="platform.admin"), 1452)
            rejected("Changing a grant to platform kind cannot retain Tenant scope", "access_grants",
                     tenant_grant(uid(214), kind="platform", permission_code="platform.admin"), 4025)
            rejected("A grant must select exactly one user or team", "access_grants",
                     tenant_grant(uid(215), group_id=ga), 4025)
            rejected("A grant cannot omit its user or team", "access_grants",
                     tenant_grant(uid(216), user_id=None), 4025)
            rejected("A grant cannot assign both a role and a direct permission", "access_grants",
                     tenant_grant(uid(217), role_id=ra, role_owner_key="t:" + ta), 4025)
            rejected("A grant cannot omit both role and permission", "access_grants",
                     tenant_grant(uid(218), permission_code=None), 4025)
            rejected("A grant cannot specify both Space and App scope", "access_grants",
                     tenant_grant(uid(219), space_id=sa, app_id=aa), 4025)
            rejected("Platform grants cannot use Tenant teams", "access_grants",
                     dict(id=uid(220), kind="platform", group_id=ga, permission_code="platform.admin"), 4025)
            rejected("An unknown action code cannot enter a grant", "access_grants",
                     tenant_grant(uid(221), permission_code="unknown.action"), 1452)

            def persistent_connection(sql, lines=1):
                process = subprocess.Popen(base + ["lastapp_schema_validation"],
                                           stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                           stderr=subprocess.PIPE, bufsize=0)
                processes.append(process)
                process.stdin.write((sql + "\n").encode())
                process.stdin.flush()
                received = b""
                deadline = time.monotonic() + 5
                with selectors.DefaultSelector() as selector:
                    selector.register(process.stdout, selectors.EVENT_READ)
                    while received.count(b"\n") < lines:
                        remaining = deadline - time.monotonic()
                        if remaining <= 0 or not selector.select(remaining):
                            raise TimeoutError("Persistent client did not produce its expected output")
                        chunk = os.read(process.stdout.fileno(), 4096)
                        if not chunk:
                            raise AssertionError("Persistent client exited before expected output")
                        received += chunk
                return process, received.decode().strip().splitlines()

            leader, result = persistent_connection(
                "START TRANSACTION; SELECT GET_LOCK('lastapp.validation.worker',0); COMMIT; "
                "SELECT IS_USED_LOCK('lastapp.validation.worker')=CONNECTION_ID();", lines=2)
            assert result == ["1", "1"]
            assert run("SELECT GET_LOCK('lastapp.validation.worker',0);") == "0"
            checks.append("The worker leadership lock survives COMMIT and excludes a second connection")
            leader.terminate()
            leader.wait(timeout=5)
            assert run("SELECT GET_LOCK('lastapp.validation.worker',1);") == "1"
            checks.append("Worker leadership becomes available when its owning connection ends")
            target_lock, result = persistent_connection("SELECT GET_LOCK('lastapp.validation.resource',0);")
            assert result == ["1"] and run("SELECT GET_LOCK('lastapp.validation.resource',0);") == "0"
            target_lock.terminate()
            target_lock.wait(timeout=5)
            assert run("SELECT GET_LOCK('lastapp.validation.resource',1);") == "1"
            checks.append("The resource advisory lock excludes overlap and releases on disconnect")
            run("DELETE FROM database_resources WHERE id=" + quote(shared_a), 1451)
            checks.append("A bound database cannot be deleted from inventory")
            run("DELETE FROM database_targets WHERE id=" + quote(target), 1451)
            checks.append("A referenced target cannot be deleted")
            run("DELETE FROM tenants WHERE id=" + quote(ta), 1451)
            checks.append("Deleting a Tenant cannot cascade through retained resources")
        except Exception as error:
            failure = str(error)
        finally:
            for process in processes:
                if process.poll() is None:
                    process.terminate()
                    process.wait(timeout=5)
            if server is not None and server.poll() is None:
                server.terminate()
                try:
                    server.wait(timeout=20)
                except subprocess.TimeoutExpired:
                    server.kill()
                    server.wait(timeout=5)
    timestamp = datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds")
    status = "PASS" if failure is None else "FAIL"
    report = [
        "# Control Plane Schema Validation", "", "**" + status + "** — " + timestamp, "",
        "Engine: " + version + ".", "",
        "The validator started its own temporary MariaDB with TCP disabled, applied the design DDL,",
        "and removed its data directory after stopping the private server. No existing server was used.", "",
        "These checks cover the DDL, authorization relationship constraints, conditional writes and SQL advisory lock primitives. They do not validate the future",
        "Go authorization decisions, handlers, OIDC flows, encryption, policy/quota enforcement or provisioning side effects.", "",
        str(len(checks)) + " completed checks:", "",
    ] + ["- " + check for check in checks]
    if failure:
        report += ["", "Failure:", "", "~~~text", failure, "~~~"]
    (HERE / "VALIDATION.md").write_text("\n".join(report) + "\n")
    print(status + ": " + str(len(checks)) + " checks; " + version)
    if failure:
        raise RuntimeError(failure)


if __name__ == "__main__":
    main()
