package service

import (
	"bufio"
	"errors"
	"github.com/bytedance/sonic"
	"github.com/sundaqiang/sdq-go/common"
	"go.uber.org/zap"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

type FastReqArg struct {
	Url          string
	Path         string
	Params       *fasthttp.Args
	Body         *fasthttp.Args
	BodyJson     any
	Method       string
	ContentType  string
	UserAgent    string
	Cookie       string
	MergedCookie bool
	Headers      *[]FastHeader
	Timeout      time.Duration
}

type FastResArg struct {
	Body       []byte
	BodyJson   any
	Cookie     string
	StatusCode int
	Header     string
}

type FastHeader struct {
	Name  string
	Value string
}

// 代理配置
func fastHTTPDialer(proxyAddr string) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		conn, err := fasthttp.Dial(proxyAddr)
		if err != nil {
			return nil, err
		}

		req := "CONNECT " + addr + " HTTP/1.1\r\n"
		// req += "Proxy-Authorization: xxx\r\n"
		req += "\r\n"

		if _, err := conn.Write([]byte(req)); err != nil {
			return nil, err
		}

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		res.SkipBody = true

		if err := res.Read(bufio.NewReader(conn)); err != nil {
			conn.Close()
			return nil, err
		}
		if res.Header.StatusCode() != 200 {
			conn.Close()
			return nil, errors.New("无法连接到该代理")
		}
		return conn, nil
	}
}

func FastResponse(reqArg *FastReqArg, resArg *FastResArg) bool {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req) // 用完需要释放资源
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp) // 用完需要释放资源

	// 配置超时
	if reqArg.Timeout > 0 {
		req.SetTimeout(reqArg.Timeout)
	} else {
		req.SetTimeout(30 * time.Second)
	}

	// 配置访问方式
	req.Header.SetMethod(reqArg.Method)

	// 配置请求的url
	fullUrl := reqArg.Url + reqArg.Path
	if reqArg.Params != nil && reqArg.Params.Len() > 0 {
		fullUrl += "?" + reqArg.Params.String()
	}
	req.SetRequestURI(fullUrl)

	// 配置userAgent
	userAgent := `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36 Edg/108.0.1462.54`
	if reqArg.UserAgent != "" {
		userAgent = reqArg.UserAgent
	}
	req.Header.Set("User-Agent", userAgent)

	// 配置cookie
	if reqArg.Cookie != "" {
		req.Header.Set("cookie", reqArg.Cookie)
	}

	// 配置其他协议头
	if reqArg.Headers != nil {
		for _, v := range *reqArg.Headers {
			if v.Name == "Referer" && v.Value == "" {
				v.Value = fullUrl
			}
			req.Header.Set(v.Name, v.Value)
		}
	}

	// 配置body和contentType
	var contentType string
	if reqArg.Body != nil {
		req.SetBody(reqArg.Body.QueryString())
		contentType = `application/x-www-form-urlencoded; charset=UTF-8`
	} else {
		if reqArg.BodyJson != nil {
			sonic.Pretouch(reflect.TypeOf(reqArg.BodyJson).Elem())
			bodyByte, _ := json.Marshal(reqArg.BodyJson)
			req.SetBody(bodyByte)
			contentType = `application/json; charset=UTF-8`
		}
	}

	// 配置contentType
	if reqArg.ContentType != "" {
		contentType = reqArg.ContentType
	}
	req.Header.SetContentType(contentType)

	// 访问接口
	if err := FastHttpClient.Do(req, resp); err != nil {
		ZapLog.Warn("FastResponse接口访问错误",
			zap.String("url", fullUrl),
			zap.String("method", reqArg.Method),
			zap.String("content_type", contentType),
			zap.String("user_agent", userAgent),
			zap.ByteString("body", reqArg.Body.QueryString()),
			zap.Reflect("body_json", reqArg.BodyJson),
			zap.Error(err),
		)
		return false
	}

	// 获取返回的cookie
	var newCookieArr []string
	resp.Header.VisitAllCookie(func(_, value []byte) {
		c := fasthttp.AcquireCookie()
		err := c.ParseBytes(value)
		if err != nil {
			ZapLog.Warn("FastResponse获取cookie失败", zap.Error(err))
			return
		}
		cName := common.Bytes2String(c.Key())
		cValue := common.Bytes2String(c.Value())
		newCookieArr = append(newCookieArr, cName+"="+cValue)
	})

	// 合并cookie
	if reqArg.MergedCookie {
		oldCookieArr := strings.Split(reqArg.Cookie, "; ")
		for _, v := range oldCookieArr {
			if !common.StringInSlice(newCookieArr, v) {
				newCookieArr = append(newCookieArr, v)
			}
		}
	}

	// 配置返回的cookie
	resArg.Cookie = strings.Join(newCookieArr, "; ")

	// 返回结果
	resArg.StatusCode = resp.StatusCode()
	resArg.Header = resp.Header.String()
	if resArg.StatusCode != 200 {
		ZapLog.Warn("FastResponse接口访问失败",
			zap.String("url", fullUrl),
			zap.String("method", reqArg.Method),
			zap.String("content_type", contentType),
			zap.String("user_agent", userAgent),
			zap.ByteString("body", reqArg.Body.QueryString()),
			zap.Reflect("body_json", reqArg.BodyJson),
		)
		return false
	}

	// 解析返回数据
	if resArg.BodyJson != nil {
		isConvert := common.Json2Struct(resp.Body(), resArg.BodyJson)
		if isConvert == nil {
			ZapLog.Info("FastResponse接口访问成功",
				zap.String("url", fullUrl),
				zap.String("method", reqArg.Method),
				zap.String("content_type", contentType),
				zap.String("user_agent", userAgent),
				zap.ByteString("body", reqArg.Body.QueryString()),
				zap.Reflect("body_json", reqArg.BodyJson),
				zap.Reflect("res", resArg.BodyJson),
			)
			return true
		}
	} else {
		resArg.Body = resp.Body()
		ZapLog.Info("FastResponse接口访问成功",
			zap.String("url", fullUrl),
			zap.String("method", reqArg.Method),
			zap.String("content_type", contentType),
			zap.String("user_agent", userAgent),
			zap.ByteString("body", reqArg.Body.QueryString()),
			zap.Reflect("body_json", reqArg.BodyJson),
			zap.ByteString("res", resArg.Body),
		)
		return true
	}
	ZapLog.Warn("FastResponse接口访问异常",
		zap.String("url", fullUrl),
		zap.String("method", reqArg.Method),
		zap.String("content_type", contentType),
		zap.String("user_agent", userAgent),
		zap.ByteString("body", reqArg.Body.QueryString()),
		zap.Reflect("body_json", reqArg.BodyJson),
	)
	return false
}

func (t *GinTracer) FastResponse(reqArg *FastReqArg, resArg *FastResArg) bool {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req) // 用完需要释放资源
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp) // 用完需要释放资源

	// 配置超时
	if reqArg.Timeout > 0 {
		req.SetTimeout(reqArg.Timeout)
	} else {
		req.SetTimeout(30 * time.Second)
	}

	// 配置访问方式
	req.Header.SetMethod(reqArg.Method)

	// 配置请求的url
	fullUrl := reqArg.Url + reqArg.Path
	if reqArg.Params != nil && reqArg.Params.Len() > 0 {
		fullUrl += "?" + reqArg.Params.String()
	}
	req.SetRequestURI(fullUrl)

	// 配置userAgent
	userAgent := `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36 Edg/108.0.1462.54`
	if reqArg.UserAgent != "" {
		userAgent = reqArg.UserAgent
	}
	req.Header.Set("User-Agent", userAgent)

	// 配置cookie
	if reqArg.Cookie != "" {
		req.Header.Set("cookie", reqArg.Cookie)
	}

	// 配置其他协议头
	if reqArg.Headers != nil {
		for _, v := range *reqArg.Headers {
			if v.Name == "Referer" && v.Value == "" {
				v.Value = fullUrl
			}
			req.Header.Set(v.Name, v.Value)
		}
	}

	// 配置body和contentType
	var contentType string
	if reqArg.Body != nil {
		req.SetBody(reqArg.Body.QueryString())
		contentType = `application/x-www-form-urlencoded; charset=UTF-8`
	} else {
		if reqArg.BodyJson != nil {
			sonic.Pretouch(reflect.TypeOf(reqArg.BodyJson).Elem())
			bodyByte, _ := json.Marshal(reqArg.BodyJson)
			req.SetBody(bodyByte)
			contentType = `application/json; charset=UTF-8`
		}
	}

	// 配置contentType
	if reqArg.ContentType != "" {
		contentType = reqArg.ContentType
	}
	req.Header.SetContentType(contentType)

	// 访问接口
	if err := FastHttpClient.Do(req, resp); err != nil {
		t.Log.Error("FastResponse接口访问错误",
			zap.String("url", fullUrl),
			zap.String("method", reqArg.Method),
			zap.String("content_type", contentType),
			zap.String("user_agent", userAgent),
			zap.Reflect("body", reqArg.Body),
			zap.Reflect("body_json", reqArg.BodyJson),
			zap.Error(err),
		)
		return false
	}

	// 获取返回的cookie
	var newCookieArr []string
	resp.Header.VisitAllCookie(func(_, value []byte) {
		c := fasthttp.AcquireCookie()
		err := c.ParseBytes(value)
		if err != nil {
			t.Log.Error("FastResponse获取cookie失败", zap.Error(err))
			return
		}
		cName := common.Bytes2String(c.Key())
		cValue := common.Bytes2String(c.Value())
		newCookieArr = append(newCookieArr, cName+"="+cValue)
	})

	// 合并cookie
	if reqArg.MergedCookie {
		oldCookieArr := strings.Split(reqArg.Cookie, "; ")
		for _, v := range oldCookieArr {
			if !common.StringInSlice(newCookieArr, v) {
				newCookieArr = append(newCookieArr, v)
			}
		}
	}

	// 配置返回的cookie
	resArg.Cookie = strings.Join(newCookieArr, "; ")

	// 返回结果
	resArg.StatusCode = resp.StatusCode()
	resArg.Header = resp.Header.String()
	if resArg.StatusCode != 200 {
		t.Log.Error("FastResponse接口访问失败",
			zap.String("url", fullUrl),
			zap.String("method", reqArg.Method),
			zap.String("content_type", contentType),
			zap.String("user_agent", userAgent),
			zap.Reflect("body", reqArg.Body),
			zap.Reflect("body_json", reqArg.BodyJson),
		)
		return false
	}

	// 解析返回数据
	if resArg.BodyJson != nil {
		isConvert := common.Json2Struct(resp.Body(), resArg.BodyJson)
		if isConvert == nil {
			t.Log.Info("FastResponse接口访问成功",
				zap.String("url", fullUrl),
				zap.String("method", reqArg.Method),
				zap.String("content_type", contentType),
				zap.String("user_agent", userAgent),
				zap.Reflect("body", reqArg.Body),
				zap.Reflect("body_json", reqArg.BodyJson),
				zap.Reflect("res", resArg.BodyJson),
			)
			return true
		}
	} else {
		resArg.Body = resp.Body()
		t.Log.Info("FastResponse接口访问成功",
			zap.String("url", fullUrl),
			zap.String("method", reqArg.Method),
			zap.String("content_type", contentType),
			zap.String("user_agent", userAgent),
			zap.Reflect("body", reqArg.Body),
			zap.Reflect("body_json", reqArg.BodyJson),
			zap.ByteString("res", resArg.Body),
		)
		return true
	}
	t.Log.Error("FastResponse接口访问异常",
		zap.String("url", fullUrl),
		zap.String("method", reqArg.Method),
		zap.String("content_type", contentType),
		zap.String("user_agent", userAgent),
		zap.Reflect("body", reqArg.Body),
		zap.Reflect("body_json", reqArg.BodyJson),
	)
	return false
}
