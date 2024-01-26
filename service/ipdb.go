package service

import (
	"github.com/ipipdotnet/ipdb-go"
	"github.com/sundaqiang/sdq-go/common"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"time"
)

func InitIpdb(path string, corn int64) bool {
	var err error
	Ipdb, err = ipdb.NewCity(path)
	if err != nil {
		return false
	}
	if corn > 0 && GoCron != nil {
		_, err = GoCron.Every(corn).Hour().StartAt(time.Unix(time.Now().Unix()+8, 0)).
			Tag("定时更新ipdb").Do(UpdateIpdb, path)
		if err != nil {
			return false
		}
	}

	return true
}

func UpdateIpdb(path string) {
	if path == "" {
		return
	}
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req) // 用完需要释放资源
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp) // 用完需要释放资源
	req.Header.SetMethod("GET")
	req.SetTimeout(30 * time.Second)
	uris := []string{
		"https://cdn.jsdelivr.net/npm/qqwry.ipdb/qqwry.ipdb",
		"https://unpkg.com/qqwry.ipdb/qqwry.ipdb",
	}
	for _, v := range uris {
		// 配置请求的url
		req.SetRequestURI(v)
		// 访问接口
		if err := FastHttpClient.Do(req, resp); err != nil {
			ZapLog.Error("获取远程ipdb失败", zap.Error(err))
			continue
		}
		break
	}
	res := resp.Body()
	err := common.CreateFile(path, &res)
	if err != nil {
		ZapLog.Error("保存远程ipdb失败", zap.Error(err))
		return
	}
	err = Ipdb.Reload(path)
	if err != nil {
		ZapLog.Error("ipdb更新失败", zap.Error(err))
		return
	}
}
