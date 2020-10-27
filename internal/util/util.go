package util

import (
	"encoding/json"
	"fmt"
	"html"
	"reflect"
	"sort"
	"strconv"
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
	m := make(map[string]interface{}, len(data))
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

func InterfaceToStr(s interface{}, escaped ...bool) (d string) {
	switch a := s.(type) {
	case string:
		d = a
	case int:
		d = strconv.FormatInt(int64(a), 10)
	case int32:
		d = strconv.FormatInt(int64(a), 10)
	case int64:
		d = strconv.FormatInt(a, 10)
	case float64:
		d = strconv.FormatFloat(a, 'f', -1, 64)
	default:
		bs, _ := json.Marshal(a)
		d = string(bs)
	}

	if len(escaped) == 1 && escaped[0] {
		d = Escape(d)
	}
	return
}
func Escape(src string) string {
	return html.EscapeString(src)
}

// 字符串false,0 会被认定为false
func InterfaceToBool(s interface{}) (d bool) {
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

func ForInterface(s interface{}, cb func(index int, v interface{}) error) error {
	switch a := s.(type) {
	case []interface{}:
		for i := range a {
			if err := cb(i, a[i]); err != nil {
				return err
			}
		}
	case []map[string]interface{}:
		for i := range a {
			if err := cb(i, a[i]); err != nil {
				return err
			}
		}
	case []int:
		for i := range a {
			if err := cb(i, a[i]); err != nil {
				return err
			}
		}
	case []int64:
		for i := range a {
			if err := cb(i, a[i]); err != nil {
				return err
			}
		}
	case []int32:
		for i := range a {
			if err := cb(i, a[i]); err != nil {
				return err
			}
		}
	case []string:
		for i := range a {
			if err := cb(i, a[i]); err != nil {
				return err
			}
		}
	case []float64:
		for i := range a {
			if err := cb(i, a[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

func Interface2Slice(s interface{}) (d []interface{}) {

	return
}
