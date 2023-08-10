package service

import (
	"errors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// BindJson 绑定数据
func BindJson(c *gin.Context, code int, body any) bool {
	if err := c.ShouldBindJSON(body); err != nil {
		if IsTrans {
			GetHttpResErrorTrans(c, http.StatusBadRequest, code, err)
			return false
		}
		GetHttpResError(c, http.StatusBadRequest, code, body, err)
		return false
	}
	return true
}

// BindForm 绑定数据
func BindForm(c *gin.Context, code int, body any) bool {
	if err := c.ShouldBindWith(body, binding.Form); err != nil {
		if IsTrans {
			GetHttpResErrorTrans(c, http.StatusBadRequest, code, err)
			return false
		}
		GetHttpResError(c, http.StatusBadRequest, code, body, err)
		return false
	}
	return true
}

// BindQuery 绑定数据
func BindQuery(c *gin.Context, code int, body any) bool {
	if err := c.ShouldBindQuery(body); err != nil {
		if IsTrans {
			GetHttpResErrorTrans(c, http.StatusBadRequest, code, err)
			return false
		}
		GetHttpResError(c, http.StatusBadRequest, code, body, err)
		return false
	}
	return true
}

// GetHttpResSuccess 封装一个正确的返回值
func GetHttpResSuccess(c *gin.Context, status, code int, data any) {
	c.JSON(
		status,
		&gin.H{
			"success":    true, // 布尔值，表示本次调用是否成功
			"code":       code,
			"request_id": requestid.Get(c),
			"data":       data, // 调用成功（success为true）时，服务端返回的数据。 不允许返回JS中undefine结果，0，false，"" 等
		},
	)
	return
}

// GetHttpResFailure 封装一个失败的返回值
func GetHttpResFailure(c *gin.Context, status, code int, msg string) {
	c.JSON(
		status,
		&gin.H{
			"success":    false, // 布尔值，表示本次调用是否成功
			"code":       code,  // 字符串，调用失败（success为false）时，服务端返回的错误码
			"request_id": requestid.Get(c),
			"msg":        msg, // 字符串，调用失败（success为false）时，服务端返回的错误信息
		},
	)
	return
}

// GetHttpResError 封装一个错误的返回值
func GetHttpResError(c *gin.Context, status, code int, data any, err error) {
	c.JSON(
		status,
		&gin.H{
			"success":    false, // 布尔值，表示本次调用是否成功
			"code":       code,  // 字符串，调用失败（success为false）时，服务端返回的错误码
			"request_id": requestid.Get(c),
			"msg":        getValidMsg(err, data), // 字符串，调用失败（success为false）时，服务端返回的错误信息
		},
	)
	return
}

// GetHttpResErrorTrans 封装一个错误的返回值,翻译
func GetHttpResErrorTrans(c *gin.Context, status, code int, err error) {
	if errors.Is(err, io.EOF) {
		GetHttpResFailure(c, http.StatusBadRequest, code, "缺少参数")
		return
	}
	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		c.JSON(
			status,
			&gin.H{
				"success":    false, // 布尔值，表示本次调用是否成功
				"code":       code,  // 字符串，调用失败（success为false）时，服务端返回的错误码
				"request_id": requestid.Get(c),
				"msg":        removeTopStruct(errs.Translate(trans)), // 字符串，调用失败（success为false）时，服务端返回的错误信息
			},
		)
		return
	}
	GetHttpResFailure(c, http.StatusBadRequest, code, "服务异常")
	return
}
