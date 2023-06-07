package common

import (
	"bytes"
	"encoding/hex"
	"go.uber.org/zap"
	"io"
	"math"
	"mime/multipart"
	"os"
	"path"
	"strconv"
	"strings"
)

// MustOpenFile 打开文件
func MustOpenFile(f string) *os.File {
	r, err := os.Open(f)
	if err != nil {
		ZapLog.Error("文件打开失败",
			zap.String("path", f),
			zap.Error(err),
		)
		return nil
	}
	return r
}

// CreateFile 创建文件
func CreateFile(fileName string, content *[]byte) (isOk bool) {
	f1, err := os.Create(fileName)
	if err != nil {
		ZapLog.Error("文件创建失败",
			zap.String("file_name", fileName),
			zap.Error(err),
		)
		return
	}
	defer func(f1 *os.File) {
		err := f1.Close()
		if err != nil {
			ZapLog.Error("文件关闭失败",
				zap.String("file_name", fileName),
				zap.Error(err),
			)
			isOk = false
			return
		}
	}(f1)
	_, err = f1.Write(*content)
	if err != nil {
		ZapLog.Error("文件写入失败",
			zap.String("file_name", fileName),
			zap.Error(err),
		)
		return
	}
	isOk = true
	return
}

/*
CreateMultipartFormData http文件上传构造器
fieldName=文件类型 fieldValue=文件名
fileName=文件路径 fieldArr=其他form参数
*/
func CreateMultipartFormData(fieldName, fieldValue string, fileName any, fieldArr *map[string]string) (bytes.Buffer, *multipart.Writer) {
	var b bytes.Buffer
	var err error
	w := multipart.NewWriter(&b)
	for k, v := range *fieldArr {
		err = w.WriteField(k, v)
		if err != nil {
			ZapLog.Error("构造MultipartFormData失败",
				zap.String("key", k),
				zap.String("value", v),
				zap.Error(err),
			)
			return bytes.Buffer{}, nil
		}
	}
	var fw io.Writer
	var file io.Reader
	switch fileName.(type) {
	case string:
		tempName := fileName.(string)
		file = MustOpenFile(tempName)
		if fieldValue == "" {
			fieldValue = path.Base(tempName)
		}
	case *[]byte:
		file = bytes.NewReader(*fileName.(*[]byte))
	}
	if fw, err = w.CreateFormFile(fieldName, fieldValue); err != nil {
		ZapLog.Error("新建文件MultipartData失败",
			zap.String("field", fieldName),
			zap.String("value", fieldValue),
			zap.Error(err),
		)
	}
	if _, err = io.Copy(fw, file); err != nil {
		ZapLog.Error("复制文件流失败",
			zap.Error(err),
		)
	}
	err = w.Close()
	if err != nil {
		ZapLog.Error("关闭文件流失败",
			zap.Error(err),
		)
		return bytes.Buffer{}, nil
	}
	return b, w
}

// GetFileType 通过文件字节流获取文件类型
func GetFileType(fSrc []byte) string {
	var fileType string
	fileCode := bytesToHexString(&fSrc)
	fileTypeMap.Range(func(key, value interface{}) bool {
		k := key.(string)
		v := value.(string)
		if strings.HasPrefix(fileCode, strings.ToLower(k)) ||
			strings.HasPrefix(k, strings.ToLower(fileCode)) {
			fileType = v
			return false
		}
		return true
	})
	return fileType
}

// FileSplit 按大小分割文件
func FileSplit(fileName string, splitSize int64, res *[][]byte) {
	file, err := os.Open(fileName)
	if err != nil {
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			return
		}
	}(file)
	fileInfo, _ := file.Stat()
	fileSize := uint64(fileInfo.Size())
	fileChunk := uint64(splitSize * (1 << 20))
	totalPartsNum := uint64(math.Ceil(float64(fileSize) / float64(fileChunk)))
	for i := uint64(1); i <= totalPartsNum; i++ {
		partSize := int(math.Min(float64(fileChunk), float64(fileSize-((i-1)*fileChunk))))
		partBuffer := make([]byte, partSize)
		_, err := file.Read(partBuffer)
		if err != nil {
			return
		}
		*res = append(*res, partBuffer)
	}
}

// 获取字节
func bytesToHexString(src *[]byte) string {
	res := bytes.Buffer{}
	if src == nil || len(*src) <= 0 {
		return ""
	}
	temp := make([]byte, 0)
	for _, v := range *src {
		sub := v & 0xFF
		hv := hex.EncodeToString(append(temp, sub))
		if len(hv) < 2 {
			res.WriteString(strconv.FormatInt(int64(0), 10))
		}
		res.WriteString(hv)
	}
	return res.String()
}
