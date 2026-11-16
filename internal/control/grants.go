package control

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
)

// RuntimeAccount is the registry account the server connects as.
const RuntimeAccount = "lastapp_control"

// appendOnlyRegistryTables are registry tables the runtime account may read and
// insert into but never update or delete: audit history, MCP receipts and
// immutable builds/releases. Every other registry table (except goose's own
// version table) receives runtime UPDATE,DELETE grants.
var appendOnlyRegistryTables = map[string]bool{
	"audit_events":             true,
	"agent_operation_receipts": true,
	"app_builds":               true,
	"app_releases":             true,
}

var (
	createdTable = regexp.MustCompile(`(?i)CREATE TABLE\s+(?:IF NOT EXISTS\s+)?` + "`?" + `([a-z0-9_]+)`)
	grantName    = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	grantHost    = regexp.MustCompile(`^[A-Za-z0-9_.:%-]+$`)
)

// registryTables lists the tables created by the embedded migrations, sorted.
func registryTables() ([]string, error) {
	names, e := fs.Glob(migrationFiles, "migrations/*.sql")
	if e != nil {
		return nil, fmt.Errorf("list registry migrations: %w", e)
	}
	seen := map[string]bool{}
	for _, name := range names {
		b, e := migrationFiles.ReadFile(name)
		if e != nil {
			return nil, fmt.Errorf("read registry migration %s: %w", name, e)
		}
		for _, m := range createdTable.FindAllStringSubmatch(string(b), -1) {
			seen[m[1]] = true
		}
	}
	for table := range appendOnlyRegistryTables {
		if !seen[table] {
			return nil, fmt.Errorf("append-only table %s is not created by the registry migrations", table)
		}
	}
	tables := make([]string, 0, len(seen))
	for table := range seen {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	return tables, nil
}

// RuntimeGrantStatements returns the UPDATE,DELETE grants for the runtime
// account on every mutable registry table. Base SELECT,INSERT on the whole
// registry is granted separately; append-only tables receive nothing more.
func RuntimeGrantStatements(database, host string) ([]string, error) {
	if !grantName.MatchString(database) {
		return nil, errors.New("invalid registry database name")
	}
	if !grantHost.MatchString(host) {
		return nil, errors.New("invalid runtime account host")
	}
	tables, e := registryTables()
	if e != nil {
		return nil, e
	}
	var statements []string
	for _, table := range tables {
		if !appendOnlyRegistryTables[table] {
			statements = append(statements, "GRANT UPDATE,DELETE ON `"+database+"`.`"+table+"` TO '"+RuntimeAccount+"'@'"+host+"'")
		}
	}
	return statements, nil
}
