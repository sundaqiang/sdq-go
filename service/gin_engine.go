package service

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

func defaultRemoteIPHeaders() []string {
	return []string{"X-Forwarded-For", "X-Real-IP"}
}

// newGinEngine 使用安全的客户端 IP 默认值；未配置可信代理时不读取转发头。
func newGinEngine() *gin.Engine {
	r := gin.New()
	// nil 只会生成空信任列表，不可能产生地址解析错误。
	_ = r.SetTrustedProxies(nil)
	r.RemoteIPHeaders = defaultRemoteIPHeaders()
	r.TrustedPlatform = ""
	return r
}

// configureGinClientIP applies the deployment's explicit client IP trust
// boundary and header list. Empty proxies keep forwarded headers untrusted.
func configureGinClientIP(r *gin.Engine, trustedProxies, remoteIPHeaders []string) error {
	cleanHeaders := make([]string, 0, len(remoteIPHeaders))
	for _, header := range remoteIPHeaders {
		header = strings.TrimSpace(header)
		if header == "" {
			return errors.New("remote-ip-headers contains an empty header name")
		}
		cleanHeaders = append(cleanHeaders, header)
	}
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return err
	}
	if len(cleanHeaders) == 0 {
		cleanHeaders = defaultRemoteIPHeaders()
	}
	r.RemoteIPHeaders = cleanHeaders
	return nil
}
