package vpl

import (
	"fmt"
	"github.com/zbysir/vpl/internal/parser"
	"github.com/zbysir/vpl/internal/util"
	"sort"
	"strings"
)

// classProps: 支持 obj, array, string
func getClassFromProps(classProps interface{}) parser.Class {
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
			if util.InterfaceToBool(v) {
				c = append(c, k)
			}
		}
		sort.Strings(c)
		cs = c
	case []interface{}:
		var c []string
		for _, v := range t {
			cc := getClassFromProps(v)
			c = append(c, cc...)
		}

		cs = c
	}

	for i := range cs {
		cs[i] = util.Escape(cs[i])
	}

	return cs
}

// 将静态props生成attr字符串
// 用于预编译
func genAttrFromProps(props parser.Props) string {
	c := strings.Builder{}
	for _, a := range props {
		if !a.IsStatic {
			continue
		}
		v := a.Val
		k := a.Key

		if c.Len() != 0 {
			c.WriteString(" ")
		}
		if v != "" {
			c.WriteString(fmt.Sprintf(`%s="%s"`, k, v))
		} else {
			c.WriteString(fmt.Sprintf(`%s`, k))
		}
	}

	return c.String()
}

// 打印Statement, 方便调试
func NicePrintStatement(st Statement, lev int) string {
	s := strings.Repeat(" ", lev)

	switch t := st.(type) {
	case *StrStatement:
		s += fmt.Sprintf("%s\n", t.Str)
	case *groupStatement:
		s = ""
		for _, v := range t.s {
			s += fmt.Sprintf("%s", NicePrintStatement(v, lev))
		}
	case *ComponentStatement:
		s += fmt.Sprintf("<%s>\n", t.ComponentKey)
		s += fmt.Sprintf("%s", NicePrintStatement(t.ComponentStruct.Slots["default"].Children, lev+1))
		s += fmt.Sprintf("<%s/>\n", t.ComponentKey)
	case *tagStatement:
		s += fmt.Sprintf("TagStart(%s, %+v)\n", t.tag, t.tagStruct)
	case *ifStatement:
		s += fmt.Sprintf("If(%+v)\n", t.conditionCode)
		s += fmt.Sprintf("%s", NicePrintStatement(t.ChildStatement, lev+1))

		for _, ef := range t.ElseIf {
			if ef.conditionCode != "" {
				s += fmt.Sprintf("ElseIf(%+v)\n", ef.conditionCode)
			} else {
				s += fmt.Sprintf("Else\n")
			}

			s += fmt.Sprintf("%s", NicePrintStatement(ef.ChildStatement, lev+1))
		}
	case *forStatement:
		s += fmt.Sprintf("For(%s in %s)\n", t.ItemKey, t.ArrayKey)
		s += fmt.Sprintf("%s", NicePrintStatement(t.ChildChunks, lev+1))
	case *mustacheStatement:
		s += fmt.Sprintf("{{%s}}\n", t.exp)
	case *rawHtmlStatement:
		s += fmt.Sprintf("{{%s}}\n", t.exp)
	default:

	}

	return s
}
