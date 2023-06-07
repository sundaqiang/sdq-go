package common

import (
	"crypto/md5"
	"encoding/hex"
	"go.uber.org/zap"
	"unsafe"
)

// Bytes2String 字节集转字符串
func Bytes2String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// GetMd5 获取md5
func GetMd5(file *[]byte) string {
	fileMd5 := md5.New()
	_, err := fileMd5.Write(*file)
	if err != nil {
		ZapLog.Error("读取MD5失败",
			zap.Error(err),
		)
		return ""
	}
	return hex.EncodeToString(fileMd5.Sum(nil))
}
