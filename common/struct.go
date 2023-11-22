package common

import (
	"errors"
	"github.com/bytedance/sonic"
	"reflect"
)

// FieldInfo 存储struct中每个字段的信息
type FieldInfo struct {
	Name      string       // 字段名
	Type      reflect.Type // 字段类型
	IsSlice   bool         // 是否是切片类型
	IsStruct  bool         // 是否是结构体类型
	ElemType  reflect.Type // 如果该字段是切片，则存储此切片的元素类型；否则为nil
	FieldPath []int        // 字段路径，便于嵌套结构体赋值
}

// TypeFieldMap 存储struct中每个类型的字段信息，以类型为key，以字段信息的slice为value
type TypeFieldMap map[reflect.Type][]FieldInfo

// fieldInfoCache 存储struct类型的字段信息缓存
var fieldInfoCache = make(TypeFieldMap)

// cacheFieldInfo 缓存struct类型的字段信息
func cacheFieldInfo(typ reflect.Type, tag string) []FieldInfo {
	var fields []FieldInfo
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name, isOmitted := field.Tag.Lookup(tag)
		if isOmitted {
			name = field.Name
		}
		var elemType reflect.Type
		if field.Type.Kind() == reflect.Slice {
			elemType = field.Type.Elem()
		} else if field.Type.Kind() == reflect.Struct {
			elemType = field.Type
		}
		fields = append(fields, FieldInfo{
			Name:      name,
			Type:      field.Type,
			IsSlice:   field.Type.Kind() == reflect.Slice,
			IsStruct:  field.Type.Kind() == reflect.Struct,
			ElemType:  elemType,
			FieldPath: []int{i},
		})
	}
	fieldInfoCache[typ] = fields
	return fields
}

// getFields 返回指定struct类型的字段信息列表
func getFields(typ reflect.Type, tag string) []FieldInfo {
	fields, ok := fieldInfoCache[typ]
	if ok {
		return fields
	}
	return cacheFieldInfo(typ, tag)
}

// Json2Struct json转结构体
func Json2Struct(jsonData any, res any) error {
	var req []byte
	switch jsonData.(type) {
	case []byte:
		req = jsonData.([]byte)
	case string:
		req = String2Bytes(jsonData.(string))
	default:
		return errors.New("不支持的数据类型")
	}
	sonic.Pretouch(reflect.TypeOf(res).Elem())
	err := json.Unmarshal(req, res)
	if err != nil {
		return err
	}
	return nil
}

// Json2Map json转Map
func Json2Map(jsonData any) *map[string]interface{} {
	var req []byte
	switch jsonData.(type) {
	case []byte:
		req = jsonData.([]byte)
	case string:
		req = []byte(jsonData.(string))
	default:
		return nil
	}
	logData := make(map[string]interface{})
	sonic.Pretouch(reflect.TypeOf(logData))
	err := json.Unmarshal(req, &logData)
	if err != nil {
		return nil
	}
	return &logData
}

// Struct2Byte 结构体转字节集
func Struct2Byte(jsonData any) ([]byte, error) {
	sonic.Pretouch(reflect.TypeOf(jsonData).Elem())
	res, err := json.Marshal(jsonData)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Struct2String 结构体转字符串
func Struct2String(jsonData any) (string, error) {
	var res []byte
	sonic.Pretouch(reflect.TypeOf(jsonData).Elem())
	res, err := json.Marshal(jsonData)
	if err != nil {
		return "{}", err
	}
	return Bytes2String(res), nil
}

// StructAssign 复制结构体
func StructAssign(target, source interface{}, tag string) {
	bVal := reflect.ValueOf(target).Elem()
	vVal := reflect.ValueOf(source).Elem()
	bTypeOfT := bVal.Type()
	vTypeOfT := vVal.Type()
	vFields := getFields(vTypeOfT, tag)
	vFieldMap := make(map[string]reflect.Value)
	for i := 0; i < len(vFields); i++ {
		vFieldMap[vFields[i].Name] = vVal.Field(i)
	}
	bFields := getFields(bTypeOfT, tag)
	for _, field := range bFields {
		if v, ok := vFieldMap[field.Name]; ok {
			if field.IsSlice {
				if !v.IsValid() || v.IsNil() {
					continue
				}
				targetSlice := reflect.MakeSlice(field.Type, v.Len(), v.Cap())
				for j := 0; j < v.Len(); j++ {
					elem := v.Index(j)
					targetElem := targetSlice.Index(j)
					if field.ElemType.Kind() == reflect.Struct {
						StructAssign(targetElem.Addr().Interface(), elem.Addr().Interface(), tag)
					} else {
						targetElem.Set(elem)
					}
				}
				bVal.FieldByIndex(field.FieldPath).Set(targetSlice)
			} else if field.IsStruct {
				StructAssign(bVal.FieldByIndex(field.FieldPath).Addr().Interface(), v.Addr().Interface(), tag)
			} else {
				if field.Name != "" {
					bVal.FieldByIndex(field.FieldPath).Set(v)
				}
			}
		}
	}
}
