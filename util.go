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
		c := make([]string, 0, len(t)/2)
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

func writeClass(classProps interface{}, w *strings.Builder) {
	if classProps == nil {
		return
	}
	switch t := classProps.(type) {
	case []string:
		for _, c := range t {
			if w.Len() != 0 {
				w.WriteString(" ")
			}
			w.WriteString(util.Escape(c))
		}
	case string:
		if w.Len() != 0 {
			w.WriteString(" ")
		}
		w.WriteString(util.Escape(t))

	case map[string]interface{}:
		c := make([]string, 0, len(t)/2)
		for k, v := range t {
			if util.InterfaceToBool(v) {
				c = append(c, k)
			}
		}
		sort.Strings(c)
		for _, c := range c {
			if w.Len() != 0 {
				w.WriteString(" ")
			}
			w.WriteString(util.Escape(c))
		}
	case []interface{}:
		for _, v := range t {
			writeClass(v, w)
		}
	}
}

// 将静态props生成attr字符串
// 用于预编译
func genAttrFromProps(props parser.Props) string {
	s := strings.Builder{}
	for _, a := range props {
		if !a.IsStatic {
			continue
		}
		if a.Key == "style" {
			sty := getStyleFromProps(a.StaticVal)
			if len(sty) != 0 {
				if s.Len() != 0 {
					s.WriteString(" ")
				}
				s.WriteString(`style="`)
				s.WriteString(sty.ToAttr())
				s.WriteString(`"`)
			}
		} else if a.Key == "class" {
			cla := getClassFromProps(a.StaticVal)
			if len(cla) != 0 {
				if s.Len() != 0 {
					s.WriteString(" ")
				}
				s.WriteString(`class="`)
				s.WriteString(cla.ToAttr())
				s.WriteString(`"`)
			}
		} else {
			if s.Len() != 0 {
				s.WriteString(" ")
			}
			v := a.StaticVal.(string)
			if v != "" {
				s.WriteString(fmt.Sprintf(`%s="%s"`, a.Key, v))
			} else {
				s.WriteString(fmt.Sprintf(`%s`, a.Key))
			}
		}
	}

	return s.String()
}

// 打印Statement, 方便调试
func NicePrintStatement(st Statement, lev int) string {
	index := strings.Repeat(" ", lev*2)

	s := ""
	switch t := st.(type) {
	case *StrStatement:
		s += fmt.Sprintf("%s%s\n", index, t.Str)
	case *groupStatement:
		s = ""
		for _, v := range t.s {
			s += fmt.Sprintf("%s", NicePrintStatement(v, lev))
		}
	case *ComponentStatement:
		s += fmt.Sprintf("%s<%s>\n", index, t.ComponentKey)
		if t.ComponentStruct.Slots.Default != nil {
			s += fmt.Sprintf("%s", NicePrintStatement(t.ComponentStruct.Slots.Default.Children, lev+1))
		}
		s += fmt.Sprintf("%s<%s/>\n", index, t.ComponentKey)
	case *tagStatement:
		s += fmt.Sprintf("%sTag(%s, %+v", index, t.tag, t.tagStruct.Props)
		if t.tagStruct.VBind != nil {
			if t.tagStruct.VBind.useProps {
				s += fmt.Sprintf(", BindProps")
			}
		}

		if len(t.tagStruct.Directives) != 0 {
			s += fmt.Sprintf(",")
			for _, v := range t.tagStruct.Directives {
				s += fmt.Sprintf(" v-%s", v.Name)
			}
		}

		s += fmt.Sprintf(")\n")

		s += fmt.Sprintf("%s", NicePrintStatement(t.tagStruct.Slots.Default.Children, lev+1))

	case *ifStatement:
		s += fmt.Sprintf("%sIf(%+v)\n", index, t.conditionCode)
		s += fmt.Sprintf("%s", NicePrintStatement(t.ChildStatement, lev+1))

		for _, ef := range t.ElseIf {
			if ef.conditionCode != "" {
				s += fmt.Sprintf("%sElseIf(%+v)\n", index, ef.conditionCode)
			} else {
				s += fmt.Sprintf("%sElse\n", index)
			}

			s += fmt.Sprintf("%s", NicePrintStatement(ef.ChildStatement, lev+1))
		}
	case *forStatement:
		s += fmt.Sprintf("%sFor(%s in %s)\n", index, t.ItemKey, t.ArrayKey)
		s += fmt.Sprintf("%s", NicePrintStatement(t.ChildChunks, lev+1))
	case *mustacheStatement:
		s += fmt.Sprintf("%s{{%s}}\n", index, t.exp)
	case *rawHtmlStatement:
		s += fmt.Sprintf("%s{{{%s}}}\n", index, t.exp)
	default:

	}

	return s
}

// 不支持json序列化的map, 解决循环引用时Marshal报错的问题
type skipMarshalMap map[string]interface{}

func (s skipMarshalMap) MarshalJSON() ([]byte, error) {
	return nil, nil
}
