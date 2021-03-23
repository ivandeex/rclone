// Package bilib provides common stuff for bisync and bisync_test
// Functions here provide simple aliases for rclone hash names.
// TODO will be later implemented in fs/hash.go
// SEE https://github.com/rclone/rclone/issues/5071
package bilib

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/hash"
)

var hash2alias map[hash.Type]string
var alias2hash map[string]hash.Type

// SetupHashAliases creates aliases for hash names
func SetupHashAliases() {
	if hash2alias != nil {
		return
	}
	hash2alias = make(map[hash.Type]string)
	alias2hash = make(map[string]hash.Type)
	for _, h := range hash.Supported().Array() {
		s := h.String()
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "-", "")
		s = strings.TrimSuffix(s, "hash")
		hash2alias[h] = s
		alias2hash[s] = h
	}
}

// HashString is a better version of hashType.String()
func HashString(h hash.Type) string {
	if h == hash.None {
		return ""
	}
	name := hash2alias[h]
	if name == "" {
		err := fmt.Sprintf("internal error: unknown hash type: 0x%x", int(h))
		panic(err)
	}
	return name
}

// HashSet is a better version of hashType.Set(s)
func HashSet(s string) (hash.Type, error) {
	if s == "" {
		return hash.None, nil
	}
	if h, ok := alias2hash[s]; ok {
		return h, nil
	}
	return hash.None, errors.Errorf("unknown hash type %q", s)
}
