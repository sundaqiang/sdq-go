package service

import (
	"errors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-co-op/gocron/v2"
	"github.com/go-playground/validator/v10"
	"github.com/orca-zhang/ecache"
	"github.com/redis/go-redis/v9"
	"github.com/sony/sonyflake"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type GinTracer struct {
	Cache *ecache.Cache
	Ctx   *gin.Context
	Cron  gocron.Scheduler
	Db    *gorm.DB
	Http  *fasthttp.Client
	Log   *zap.Logger
	Rdb   []*redis.Client
	Sony  *sonyflake.Sonyflake
	Tid   string
}

// GetGinTracer 获取上下文实例
func GetGinTracer(c *gin.Context) *GinTracer {
	var db *gorm.DB
	db = Db
	if Db != nil {
		db = Db.WithContext(c)
	}
	return &GinTracer{
		Cache: LRUCache,
		Cron:  GoCron,
		Ctx:   c,
		Db:    db,
		Http:  FastHttpClient,
		Log:   ZapLog.With(zap.String(config.Server.Trace, requestid.Get(c))),
		Rdb:   Rdb,
		Sony:  SonyFlake,
		Tid:   requestid.Get(c),
	}
}

// BindJson 绑定数据
func (t *GinTracer) BindJson(code int, body any) bool {
	if err := t.Ctx.ShouldBindJSON(body); err != nil {
		if config.Server.Trans {
			t.GetHttpResErrorTrans(http.StatusOK, code, err)
			return false
		}
		t.GetHttpResError(http.StatusOK, code, body, err)
		return false
	}
	return true
}

// BindForm 绑定数据
func (t *GinTracer) BindForm(code int, body any) bool {
	if err := t.Ctx.ShouldBindWith(body, binding.Form); err != nil {
		if config.Server.Trans {
			t.GetHttpResErrorTrans(http.StatusOK, code, err)
			return false
		}
		t.GetHttpResError(http.StatusOK, code, body, err)
		return false
	}
	return true
}

// BindQuery 绑定数据
func (t *GinTracer) BindQuery(code int, body any) bool {
	if err := t.Ctx.ShouldBindQuery(body); err != nil {
		if config.Server.Trans {
			t.GetHttpResErrorTrans(http.StatusOK, code, err)
			return false
		}
		t.GetHttpResError(http.StatusOK, code, body, err)
		return false
	}
	return true
}

// GetHttpResSuccess 封装一个正确的返回值
func (t *GinTracer) GetHttpResSuccess(status, code int, data any) {
	t.Ctx.JSON(
		status,
		&gin.H{
			"success":           true, // 布尔值，表示本次调用是否成功
			"code":              code,
			config.Server.Trace: t.Tid,
			"data":              data, // 调用成功（success为true）时，服务端返回的数据。 不允许返回JS中undefine结果，0，false，"" 等
		},
	)
	return
}

// GetHttpResFailure 封装一个失败的返回值
func (t *GinTracer) GetHttpResFailure(status, code int, msg any) {
	t.Ctx.AbortWithStatusJSON(
		status,
		&gin.H{
			"success":           false, // 布尔值，表示本次调用是否成功
			"code":              code,  // 字符串，调用失败（success为false）时，服务端返回的错误码
			config.Server.Trace: t.Tid,
			"msg":               msg, // 字符串，调用失败（success为false）时，服务端返回的错误信息
		},
	)
	return
}

// GetHttpResError 封装一个错误的返回值
func (t *GinTracer) GetHttpResError(status, code int, data any, err error) {
	t.Ctx.AbortWithStatusJSON(
		status,
		&gin.H{
			"success":           false, // 布尔值，表示本次调用是否成功
			"code":              code,  // 字符串，调用失败（success为false）时，服务端返回的错误码
			config.Server.Trace: t.Tid,
			"msg":               getValidMsg(err, data), // 字符串，调用失败（success为false）时，服务端返回的错误信息
		},
	)
	return
}

// GetHttpResErrorTrans 封装一个错误的返回值,翻译
func (t *GinTracer) GetHttpResErrorTrans(status, code int, err error) {
	if errors.Is(err, io.EOF) {
		t.GetHttpResFailure(http.StatusOK, code, "缺少参数")
		return
	}
	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		t.Ctx.AbortWithStatusJSON(
			status,
			&gin.H{
				"success":           false, // 布尔值，表示本次调用是否成功
				"code":              code,  // 字符串，调用失败（success为false）时，服务端返回的错误码
				config.Server.Trace: t.Tid,
				"msg":               removeTopStruct(errs.Translate(trans)), // 字符串，调用失败（success为false）时，服务端返回的错误信息
			},
		)
		return
	}
	if strings.Contains(err.Error(), "cannot unmarshal") {
		t.GetHttpResFailure(http.StatusOK, code, "参数异常")
		return
	}
	t.GetHttpResFailure(http.StatusOK, code, "服务异常")
	return
}
