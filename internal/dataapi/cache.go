package dataapi

import (
	"fmt"
	"sync"

	"github.com/aantich/lastapp/internal/entity"
)

type definitionCache struct {
	mu      sync.Mutex
	entries map[string]cachedDefinition
	tick    uint64
}
type cachedDefinition struct {
	definition entity.Definition
	used       uint64
}

func (c *definitionCache) get(key, hash, source string) (entity.Definition, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tick++
	if v, ok := c.entries[key]; ok {
		v.used = c.tick
		c.entries[key] = v
		return v.definition, nil
	}
	d, e := entity.Decode([]byte(source))
	if e != nil {
		return d, e
	}
	if entity.Hash(d) != hash {
		return d, fmt.Errorf("Published definition hash mismatch")
	}
	if c.entries == nil {
		c.entries = map[string]cachedDefinition{}
	}
	if len(c.entries) >= 128 {
		oldest := ""
		var used uint64 = ^uint64(0)
		for k, v := range c.entries {
			if v.used < used {
				oldest, used = k, v.used
			}
		}
		delete(c.entries, oldest)
	}
	c.entries[key] = cachedDefinition{d, c.tick}
	return d, nil
}

func (c *definitionCache) peek(key string) (entity.Definition, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	if ok {
		c.tick++
		v.used = c.tick
		c.entries[key] = v
	}
	return v.definition, ok
}
