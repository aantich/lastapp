package dataapi

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aantich/lastapp/internal/entity"
)

func TestDefinitionCache(t *testing.T) {
	source := func(key string) (string, string) {
		d := entity.Definition{Key: key, Label: key}
		b, e := json.Marshal(d)
		if e != nil {
			t.Fatal(e)
		}
		decoded, e := entity.Decode(b)
		if e != nil {
			t.Fatal(e)
		}
		return string(b), entity.Hash(decoded)
	}
	var c definitionCache
	if _, ok := c.peek("v1:h"); ok {
		t.Fatal("empty cache hit")
	}
	src, hash := source("company")
	if _, e := c.get("bad", "not-the-hash", src); e == nil {
		t.Fatal("hash mismatch accepted")
	}
	if _, ok := c.peek("bad"); ok {
		t.Fatal("mismatched definition cached")
	}
	d, e := c.get("v1:"+hash, hash, src)
	if e != nil || d.Key != "company" {
		t.Fatal(d, e)
	}
	// A cached entry is served without decoding the supplied source again.
	if d, e = c.get("v1:"+hash, hash, "{"); e != nil || d.Key != "company" {
		t.Fatal(d, e)
	}
	// The least recently used entry is evicted at 128 entries.
	for i := 0; i < 127; i++ {
		s, h := source(fmt.Sprintf("e%d", i))
		if _, e = c.get(fmt.Sprintf("k%d", i), h, s); e != nil {
			t.Fatal(e)
		}
	}
	if _, ok := c.peek("k0"); !ok {
		t.Fatal("entry evicted below capacity")
	}
	if _, ok := c.peek("v1:" + hash); !ok {
		t.Fatal("entry evicted below capacity")
	}
	s, h := source("overflow")
	if _, e = c.get("overflow", h, s); e != nil {
		t.Fatal(e)
	}
	if len(c.entries) != 128 {
		t.Fatalf("cache holds %d entries", len(c.entries))
	}
	if _, ok := c.peek("k1"); ok {
		t.Fatal("least recently used entry retained")
	}
	if _, ok := c.peek("k0"); !ok {
		t.Fatal("recently peeked entry evicted")
	}
}
