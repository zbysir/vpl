package compiler

import (
	"github.com/zbysir/vpl/internal/parser"
	"sort"
)

func interface2Slice(s interface{}) (d []interface{}) {
	switch a := s.(type) {
	case []interface{}:
		return a
	case []map[string]interface{}:
		d = make([]interface{}, len(a))
		for i, v := range a {
			d[i] = v
		}
	case []int:
		d = make([]interface{}, len(a))
		for i, v := range a {
			d[i] = v
		}
	case []int64:
		d = make([]interface{}, len(a))
		for i, v := range a {
			d[i] = v
		}
	case []int32:
		d = make([]interface{}, len(a))
		for i, v := range a {
			d[i] = v
		}
	case []string:
		d = make([]interface{}, len(a))
		for i, v := range a {
			d[i] = v
		}
	case []float64:
		d = make([]interface{}, len(a))
		for i, v := range a {
			d[i] = v
		}
	}
	return
}

// classProps: 支持 obj, array, string
func GetClassFromProps(classProps interface{}) parser.Class {
	if classProps == nil {
		return nil
	}
	var cs []string
	switch t := classProps.(type) {
	case []string:
		cs = t
	case string:
		cs = []string{t}
	case map[string]interface{}:
		var c []string
		for k, v := range t {
			if interfaceToBool(v) {
				c = append(c, k)
			}
		}
		sort.Strings(c)
		cs = c
	case []interface{}:
		var c []string
		for _, v := range t {
			cc := GetClassFromProps(v)
			c = append(c, cc...)
		}

		cs = c
	}

	for i := range cs {
		cs[i] = escape(cs[i])
	}

	return cs
}