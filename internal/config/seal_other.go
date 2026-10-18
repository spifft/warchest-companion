//go:build !windows

package config

// Outside Windows (used only for development and tests) the key is stored as is, in a file only this user can read.
func seal(plain []byte) ([]byte, error)    { return append([]byte(nil), plain...), nil }
func unseal(sealed []byte) ([]byte, error) { return append([]byte(nil), sealed...), nil }
