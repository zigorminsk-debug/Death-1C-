//go:build windows

package config

import (
	"encoding/base64"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const dpapiPrefix = "dpapi:"
const cryptprotectUIForbidden = 0x1

var (
	dllCrypt32             = syscall.NewLazyDLL("crypt32.dll")
	dllKernel32            = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtectData   = dllCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = dllCrypt32.NewProc("CryptUnprotectData")
	procLocalFree          = dllKernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) *dataBlob {
	if len(d) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{
		cbData: uint32(len(d)),
		pbData: &d[0],
	}
}

func (b *dataBlob) bytes() []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	src := unsafe.Slice(b.pbData, b.cbData)
	out := make([]byte, len(src))
	copy(out, src)
	return out
}

// encryptSecret encrypts a secret string for the current Windows user via DPAPI.
func encryptSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	plainBytes := []byte(plain)
	inBlob := newBlob(plainBytes)
	var outBlob dataBlob

	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(inBlob)),
		0,
		0,
		0,
		0,
		cryptprotectUIForbidden,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	runtime.KeepAlive(plainBytes)
	if r == 0 {
		return "", fmt.Errorf("ошибка шифрования DPAPI: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))

	return dpapiPrefix + base64.StdEncoding.EncodeToString(outBlob.bytes()), nil
}

// decryptSecret decrypts a DPAPI-protected secret string for the current Windows user.
func decryptSecret(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	rawB64 := strings.TrimPrefix(encoded, dpapiPrefix)
	cipherBytes, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования сохранённого пароля: %w", err)
	}
	if len(cipherBytes) == 0 {
		return "", nil
	}

	inBlob := newBlob(cipherBytes)
	var outBlob dataBlob

	r, _, callErr := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(inBlob)),
		0,
		0,
		0,
		0,
		cryptprotectUIForbidden,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	runtime.KeepAlive(cipherBytes)
	if r == 0 {
		return "", fmt.Errorf("ошибка расшифровки DPAPI: %w", callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))

	return string(outBlob.bytes()), nil
}
