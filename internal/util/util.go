package util

import (
	"fmt"
	"html"
	"reflect"
	"sort"
	"unsafe"
)

func GetSortedKey(m interface{}) (keys []string) {
	switch t := m.(type) {
	case map[string]string:
		keys = make([]string, len(t))
		index := 0
		for k := range t {
			keys[index] = k
			index++
		}
		if len(t) < 2 {
			return keys
		}

		sort.Strings(keys)
	case map[string]interface{}:
		keys = make([]string, len(t))
		index := 0
		for k := range t {
			keys[index] = k
			index++
		}
		if len(t) < 2 {
			return keys
		}

		sort.Strings(keys)
	default:
		panic(fmt.Sprintf("不支持的类型 %T, %+v", m, m))
	}

	return
}

func CopyMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	m := map[string]interface{}{}
	for k, v := range data {
		m[k] = v
	}
	return m
}

func UnsafeStrToBytes(s string) (b []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return b
}

func UnsafeBytesToStr(z []byte) string {
	return *(*string)(unsafe.Pointer(&z))
}

func escape(src string) string {
	return html.EscapeString(src)
}

// 字符串false,0 会被认定为false
func interfaceToBool(s interface{}) (d bool) {
	if s == nil {
		return false
	}
	switch a := s.(type) {
	case bool:
		return a
	case int:
		return a != 0
	case int8:
		return a != 0
	case int16:
		return a != 0
	case int32:
		return a != 0
	case int64:
		return a != 0
	case float64:
		return a != 0
	case float32:
		return a != 0
	case string:
		return a != "" && a != "false" && a != "0"
	default:
		return true
	}
}
