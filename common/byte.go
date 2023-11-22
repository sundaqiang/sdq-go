package common

import (
	"crypto/md5"
	"encoding/hex"
	"unsafe"
)

// Bytes2String 字节集转字符串
func Bytes2String(b []byte) string {
	//return *(*string)(unsafe.Pointer(&b))
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// ByteGetMd5 获取md5
func ByteGetMd5(bytes *[]byte) (string, error) {
	fileMd5 := md5.New()
	_, err := fileMd5.Write(*bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(fileMd5.Sum(nil)), nil
}

// StringGetMd5 获取md5
func StringGetMd5(str string) (string, error) {
	fileMd5 := md5.New()
	_, err := fileMd5.Write(String2Bytes(str))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(fileMd5.Sum(nil)), nil
}
