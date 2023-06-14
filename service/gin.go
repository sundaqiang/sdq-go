package service

import (
	"errors"
	"github.com/go-playground/validator/v10"
	"io"
	"reflect"

	"github.com/gin-gonic/gin"
)

// 判断哪些参数异常，并且返回结构的tag内的msg
func getValidMsg(err error, obj interface{}) string {
	if errors.Is(err, io.EOF) {
		return "缺少参数"
	}
	getObj := reflect.TypeOf(obj)
	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errs {
			if f, exist := getObj.Elem().FieldByName(e.Field()); exist {
				return f.Tag.Get("msg")
			}
		}
	}
	return err.Error()
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
