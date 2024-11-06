package common

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type CharSet struct {
	kind    int
	charset string
}

// 预定义的字符集
var charSets = []CharSet{
	{1, "0123456789"},
	{2, "abcdefghijklmnopqrstuvwxyz"},
	{4, "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
}

// String2Bytes 字符串转字节集
func String2Bytes(s string) []byte {
	/*	x := (*[2]uintptr)(unsafe.Pointer(&s))
		h := [3]uintptr{x[0], x[1], x[1]}
		return *(*[]byte)(unsafe.Pointer(&h))*/
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

/*
CreateRandomStr 生成随机字符串

	length:字符串长度
	kind:1=数字、2=小写字母、3=小写字母+数字、4=大写字母、5=大写字母+数字、6=大小写字母、7=大小写字母+数字
	customCharset:自定义字符集
*/
func CreateRandomStr(length, kind int, customCharset string) string {
	var charset string

	if customCharset != "" {
		charset = customCharset
	} else {
		for _, cs := range charSets {
			if kind&cs.kind != 0 {
				charset += cs.charset
			}
		}
	}

	if charset == "" {
		return ""
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomString := make([]byte, length)
	for i := range randomString {
		randomString[i] = charset[r.Intn(len(charset))]
	}

	return Bytes2String(randomString)
}

// IsNum 字符串是否为数字
func IsNum(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// GetBeforeStr 取前后文本，str=原文本，substr=分割文本，side=false左，blank=false找不到返回原文本
func GetBeforeStr(str string, substr string, side bool, blank bool) string {
	var n int
	runes := []rune(str)
	n = UnicodeIndex(str, substr, side)
	if n == -1 {
		if blank {
			return ""
		} else {
			return str
		}
	}
	if side {
		str = string(runes[n:])
	} else {
		str = string(runes[:n])
	}
	return str
}

// GetBetweenStr 取中间文本，blank=false找不到返回原文本
func GetBetweenStr(str, start, end string, blank bool) string {
	var n int
	n = strings.Index(str, start)
	if n == -1 {
		if blank {
			return ""
		} else {
			return str
		}
	} else {
		n = n + len(start) // 增加了else，不加的会把start带上
	}
	str2 := str[n:]
	m := strings.Index(str2, end)
	if m == -1 {
		if blank {
			return ""
		} else {
			return str
		}
	}
	str2 = str2[:m]
	return str2
}

// UnicodeIndex 取文本位置，不分中英文
func UnicodeIndex(str, substr string, side bool) int {
	result := strings.Index(str, substr)
	if result >= 0 {
		tempStr := []rune(substr)
		prefix := []byte(str)[0:result]
		rs := []rune(string(prefix))
		if side {
			result = len(rs) + len(tempStr)
		} else {
			result = len(rs)
		}
	}
	return result
}

// SnakeString 驼峰转蛇形 XxYy to xx_yy , XxYY to xx_y_y
func SnakeString(s string) string {
	data := make([]byte, 0, len(s)*2)
	j := false
	num := len(s)
	for i := 0; i < num; i++ {
		d := s[i]
		if i > 0 && d >= 'A' && d <= 'Z' && j {
			data = append(data, '_')
		}
		if i > 0 && d >= '0' && d <= '9' && j {
			data = append(data, '_')
		}
		if d != '_' {
			j = true
		}
		data = append(data, d)
	}
	return strings.ToLower(string(data[:]))
}

// CamelString 蛇形转驼峰 xx_yy to XxYx  xx_y_y to XxYY
func CamelString(s string) string {
	data := make([]byte, 0, len(s))
	j := false
	k := false
	num := len(s) - 1
	for i := 0; i <= num; i++ {
		d := s[i]
		if k == false && d >= 'A' && d <= 'Z' {
			k = true
		}
		if d >= 'a' && d <= 'z' && (j || k == false) {
			d = d - 32
			j = false
			k = true
		}
		if k && d == '_' && num > i && s[i+1] >= 'a' && s[i+1] <= 'z' {
			j = true
			continue
		}
		if k && d == '_' && num > i && s[i+1] >= '0' && s[i+1] <= '9' {
			j = true
			continue
		}
		data = append(data, d)
	}
	return string(data[:])
}

// KebabString 蛇形转KebabCase xx_yy to Xx-Yx
func KebabString(input string) string {
	// 使用 cases.Title 将字符串转换为标题格式
	t := cases.Title(language.Und)
	result := t.String(input)
	// 使用 strings.Replace 将下划线替换为连字符
	result = strings.Replace(result, "_", "-", -1)
	return result
}
