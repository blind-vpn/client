package main

import (
	"syscall"
	"unsafe"
)

var (
	dllCrypt32  = syscall.NewLazyDLL("crypt32.dll")
	dllKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procEncrypt = dllCrypt32.NewProc("CryptProtectData")
	procDecrypt = dllCrypt32.NewProc("CryptUnprotectData")
	procFree    = dllKernel32.NewProc("LocalFree")
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
		pbData: &d[0],
		cbData: uint32(len(d)),
	}
}

// encryptCredential encrypts data using Windows DPAPI (current user scope).
func encryptCredential(plaintext []byte) ([]byte, error) {
	var outBlob dataBlob
	r, _, err := procEncrypt.Call(
		uintptr(unsafe.Pointer(newBlob(plaintext))),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, err
	}
	defer procFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))

	out := make([]byte, outBlob.cbData)
	copy(out, unsafe.Slice(outBlob.pbData, outBlob.cbData))
	return out, nil
}

// decryptCredential decrypts DPAPI-encrypted data.
func decryptCredential(ciphertext []byte) ([]byte, error) {
	var outBlob dataBlob
	r, _, err := procDecrypt.Call(
		uintptr(unsafe.Pointer(newBlob(ciphertext))),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, err
	}
	defer procFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))

	out := make([]byte, outBlob.cbData)
	copy(out, unsafe.Slice(outBlob.pbData, outBlob.cbData))
	return out, nil
}
