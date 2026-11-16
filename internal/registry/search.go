package registry

import (
	"strings"
)

// SearchFields is the lower-cased text an inventory search matches: an item's
// id, label, display name, key, entity key, code and description.
func SearchFields(fields ...string) string { return strings.ToLower(strings.Join(fields, " ")) }

func (v Row) SearchText() string {
	return SearchFields(Str(v["id"]), Str(v["label"]), Str(v["display_name"]), Str(v["key"]), Str(v["entity_key"]), Str(v["code"]), Str(v["description"]))
}
