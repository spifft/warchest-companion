//go:build windows

package config

import (
	"errors"
	"syscall"
	"unsafe"
)

// Windows DPAPI (CryptProtectData): the data can only be decrypted by the same Windows user on this PC.
var (
	crypt32              = syscall.NewLazyDLL("crypt32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtectData = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect   = crypt32.NewProc("CryptUnprotectData")
	procLocalFree        = kernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func blob(b []byte) *dataBlob {
	if len(b) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{cbData: uint32(len(b)), pbData: &b[0]}
}

func (b *dataBlob) bytes() []byte {
	out := make([]byte, b.cbData)
	copy(out, unsafe.Slice(b.pbData, b.cbData))
	return out
}

const cryptprotectUIForbidden = 0x1

var entropy = []byte("Warchest Companion device key")

func seal(plain []byte) ([]byte, error) {
	var out dataBlob
	r, _, err := procCryptProtectData.Call(uintptr(unsafe.Pointer(blob(plain))), 0, uintptr(unsafe.Pointer(blob(entropy))), 0, 0,
		cryptprotectUIForbidden, uintptr(unsafe.Pointer(&out)))
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}

func unseal(sealed []byte) ([]byte, error) {
	if len(sealed) == 0 {
		return nil, errors.New("empty")
	}
	var out dataBlob
	r, _, err := procCryptUnprotect.Call(uintptr(unsafe.Pointer(blob(sealed))), 0, uintptr(unsafe.Pointer(blob(entropy))), 0, 0,
		cryptprotectUIForbidden, uintptr(unsafe.Pointer(&out)))
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}
