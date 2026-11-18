package delivery

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The deployment states the console shows come from DeploymentSteps and
// DeploymentFailed; they must be exactly the states the registry allows.
func TestDeploymentStatesMatchSchema(t *testing.T) {
	b, err := os.ReadFile("../../design/control_plane.sql")
	if err != nil {
		t.Fatal(err)
	}
	table := regexp.MustCompile(`(?s)CREATE TABLE release_deployments \((.*?)\n\)`).FindSubmatch(b)
	if table == nil {
		t.Fatal("release_deployments table not found")
	}
	check := regexp.MustCompile(`CHECK\(state IN \(([^)]*)\)\)`).FindSubmatch(table[1])
	if check == nil {
		t.Fatal("release_deployments state CHECK not found")
	}
	var schema []string
	for _, v := range strings.Split(string(check[1]), ",") {
		schema = append(schema, strings.Trim(strings.TrimSpace(v), "'"))
	}
	got := append(append([]string{}, DeploymentSteps...), DeploymentFailed)
	sort.Strings(schema)
	sort.Strings(got)
	if !reflect.DeepEqual(schema, got) {
		t.Fatalf("deployment states %v, schema allows %v", got, schema)
	}
}
