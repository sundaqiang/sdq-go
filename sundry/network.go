package sundry

import (
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"net"
	"regexp"
	"strings"
	"time"
)

// CheckNetwork 检查是否有网络
func CheckNetwork() bool {
	var dnsServers = []string{
		"223.5.5.5",
		"223.6.6.6",
		"119.29.29.29",
		"114.114.114.114",
	}
	var conn net.Conn
	var err error
	for _, dns := range dnsServers {
		conn, err = net.DialTimeout("udp", net.JoinHostPort(dns, "53"), time.Second*5)
		if err != nil {
			zapLog.Error("无法连接DNS",
				zap.String("dns", dns),
				zap.Error(err),
			)
			continue
		}
		zapLog.Info("已连接上DNS",
			zap.String("dns", dns),
		)
		break
	}
	if conn != nil {
		defer func(conn net.Conn) {
			err := conn.Close()
			if err != nil {
				zapLog.Error("关闭DialTimeout失败",
					zap.Error(err),
				)
			}
		}(conn)
		return true
	}
	return false
}

// GetLocalIP4 获取本机内网ipv4
func GetLocalIP4() string {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		zapLog.Error("获取本地ip失败", zap.Error(err))
		return ""
	}

	for i := 0; i < len(netInterfaces); i++ {
		if (netInterfaces[i].Flags & net.FlagUp) != 0 {
			addRs, _ := netInterfaces[i].Addrs()
			for _, address := range addRs {
				if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					// 获取IPv4
					if ipNet.IP.To4() != nil {
						if strings.HasPrefix(ipNet.IP.String(), "172") ||
							strings.HasPrefix(ipNet.IP.String(), "192") {
							return ipNet.IP.String()
						}
					}
				}
			}
		}
	}
	return ""
}

// GetExternalIP4 获取本机外网ipv4
func GetExternalIP4() (ip string) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req) // 用完需要释放资源
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp) // 用完需要释放资源
	ipUrls := []string{
		"https://myip.ipip.net",
		"https://myexternalip.com/raw",
		"https://ipinfo.io/ip",
	}
	for _, ipUrl := range ipUrls {
		req.SetRequestURI(ipUrl)
		req.Header.SetMethod("GET")
		if err := httpClient.Do(req, resp); err != nil {
			zapLog.Error("获取ip访问异常",
				zap.String("url", ipUrl),
				zap.Error(err),
			)
		} else {
			res := resp.String()
			ip = MatchIp(res)
			if ip == "" {
				zapLog.Error("获取ip访问错误",
					zap.String("url", ipUrl),
					zap.String("result", res),
				)
			} else {
				return
			}
		}
	}
	return
}

// MatchIp 判断字符串是否是ipv4或ipv6
func MatchIp(str string) (ipMatch string) {
	// 定义匹配IPv4地址的正则表达式
	ipRegex := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipMatch = ipRegex.FindString(str)
	if ipMatch != "" {
		return
	}
	// 定义匹配IPv6地址的正则表达式
	ipRegex = regexp.MustCompile(`\b(?:(?:(?:[a-f0-9]{1,4}:){6}|::(?:[a-f0-9]{1,4}:){0,5})((?:[a-f0-9]{1,4}:){2,7}[a-f0-9]{1,4}|(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)))\b`)
	ipMatch = ipRegex.FindString(str)
	return
}
