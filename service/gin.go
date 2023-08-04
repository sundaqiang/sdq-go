package service

import (
	"github.com/go-playground/validator/v10"
	"net/http"

	"github.com/gin-gonic/gin"
)

// BindJson 绑定数据
func BindJson(c *gin.Context, code int, body any) bool {
	if err := c.ShouldBindJSON(body); err != nil {
		c.JSON(
			http.StatusBadRequest,
			GetHttpResError(code, body, err),
		)
		return false
	}
	return true
}

// BindQuery 绑定数据
func BindQuery(c *gin.Context, code int, body any) bool {
	if err := c.ShouldBindQuery(body); err != nil {
		c.JSON(
			http.StatusBadRequest,
			GetHttpResError(code, body, err),
		)
		return false
	}
	return true
}

// GetHttpResSuccess 封装一个正确的返回值
func GetHttpResSuccess(code int, data any) *gin.H {
	return &gin.H{
		"success": true, // 布尔值，表示本次调用是否成功
		"code":    code,
		"data":    data, // 调用成功（success为true）时，服务端返回的数据。 不允许返回JS中undefine结果，0，false，"" 等
	}
}

// GetHttpResFailure 封装一个失败的返回值
func GetHttpResFailure(code int, msg string) *gin.H {
	return &gin.H{
		"success": false, // 布尔值，表示本次调用是否成功
		"code":    code,  // 字符串，调用失败（success为false）时，服务端返回的错误码
		"msg":     msg,   // 字符串，调用失败（success为false）时，服务端返回的错误信息
	}
}

// GetHttpResError 封装一个错误的返回值
func GetHttpResError(code int, data any, err error) *gin.H {
	return &gin.H{
		"success": false,                  // 布尔值，表示本次调用是否成功
		"code":    code,                   // 整数型，调用失败（success为false）时，服务端返回的错误码
		"msg":     getValidMsg(err, data), // 字符串，调用失败（success为false）时，服务端返回的错误信息
	}
}

// GetHttpResErrorTrans 封装一个错误的返回值,翻译
func GetHttpResErrorTrans(code int, errs *validator.ValidationErrors) *gin.H {
	return &gin.H{
		"success": false,                                  // 布尔值，表示本次调用是否成功
		"code":    code,                                   // 整数型，调用失败（success为false）时，服务端返回的错误码
		"msg":     removeTopStruct(errs.Translate(trans)), // 字符串，调用失败（success为false）时，服务端返回的错误信息
	}
}

// ginCors 跨域请求中间件
func ginCors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", "*") // 可将将 * 替换为指定的域名
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	}
}
