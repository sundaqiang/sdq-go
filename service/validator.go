package service

import (
	"errors"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"io"
	"reflect"
	"regexp"
	"strings"
)

var defaultRegs = ValidatorReg{
	Struct: nil,
	Field: []ValidatorField{
		{
			Tag: "validatorUserName",
			Fn:  ValidatorUserName,
		},
		{
			Tag: "validatorPassword",
			Fn:  ValidatorPassword,
		},
	},
	Translator: []ValidatorTranslator{
		{
			Tag: "validatorUserName",
			Msg: "{0}首位必须为大小写字母,只能包含大小写字母|数字且长度在6-10位字符",
		},
		{
			Tag: "validatorPassword",
			Msg: "{0}必须包含大小写字母|数字|标点符号(.@$!%*#_~?&^)至少3种的组合且长度在8-20位字符",
		},
	},
}

type ValidatorReg struct {
	Struct     []ValidatorStruct
	Field      []ValidatorField
	Translator []ValidatorTranslator
}

type ValidatorStruct struct {
	Fn    func(sl validator.StructLevel)
	Types []any
}

type ValidatorField struct {
	Tag string
	Fn  func(fl validator.FieldLevel) bool
}

type ValidatorTranslator struct {
	Tag string
	Msg string
}

// InitTrans 初始化翻译器
func initValidator(locale string) (err error) {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		for _, structs := range defaultRegs.Struct {
			v.RegisterStructValidation(structs.Fn, structs.Types)
		}
		for _, structs := range ValidatorRegs.Struct {
			v.RegisterStructValidation(structs.Fn, structs.Types)
		}
		for _, field := range defaultRegs.Field {
			if err := v.RegisterValidation(field.Tag, field.Fn); err != nil {
				return err
			}
		}
		for _, field := range ValidatorRegs.Field {
			if err := v.RegisterValidation(field.Tag, field.Fn); err != nil {
				return err
			}
		}
		// 注册一个获取json tag的自定义方法
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
		zhT := zh.New()
		enT := en.New()
		uni := ut.New(enT, zhT, enT)
		var ok bool
		trans, ok = uni.GetTranslator(locale)
		if !ok {
			return errors.New("初始化翻译器错误")
		}
		switch locale {
		case "en":
			err = enTranslations.RegisterDefaultTranslations(v, trans)
		case "zh":
			err = zhTranslations.RegisterDefaultTranslations(v, trans)
		default:
			err = enTranslations.RegisterDefaultTranslations(v, trans)
		}
		for _, translator := range defaultRegs.Translator {
			if err := v.RegisterTranslation(
				translator.Tag,
				trans,
				registerTranslator(translator.Tag, translator.Msg),
				translate,
			); err != nil {
				return err
			}
		}
		for _, translator := range ValidatorRegs.Translator {
			if err := v.RegisterTranslation(
				translator.Tag,
				trans,
				registerTranslator(translator.Tag, translator.Msg),
				translate,
			); err != nil {
				return err
			}
		}
		return
	}
	return
}

// 去除验证Struct前缀
func removeTopStruct(fields map[string]string) map[string]string {
	res := map[string]string{}
	for field, err := range fields {
		res[field[strings.Index(field, ".")+1:]] = err
	}
	return res
}

// registerTranslator 为自定义字段添加翻译功能
func registerTranslator(tag string, msg string) validator.RegisterTranslationsFunc {
	return func(trans ut.Translator) error {
		if err := trans.Add(tag, msg, false); err != nil {
			return err
		}
		return nil
	}
}

// translate 自定义字段的翻译方法
func translate(trans ut.Translator, fe validator.FieldError) string {
	msg, err := trans.T(fe.Tag(), fe.Field())
	if err != nil {
		panic(fe.(error).Error())
	}
	return msg
}

// ValidatorUserName 校验用户名
func ValidatorUserName(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	match, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9]{5,9}", value)
	return match
}

// ValidatorPassword 密码校验规则: 必须包含数字、大写字母、小写字母、特殊字符(如.@$!%*#_~?&^)至少3种的组合且长度在8-20之间
func ValidatorPassword(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	match, _ := regexp.MatchString(`^[a-zA-Z0-9.@$!%*#_~?&^]{8,20}$`, value)
	if !match {
		return false
	}
	var level = 0
	patternList := []string{`[0-9]+`, `[a-z]+`, `[A-Z]+`, `[.@$!%*#_~?&^]+`}
	for _, pattern := range patternList {
		match, _ := regexp.MatchString(pattern, value)
		if match {
			level++
		}
	}
	if level < 3 {
		return false
	}
	return true
}

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
