package parser

import (
	"errors"
	"fmt"
	"github.com/zbysir/vpl/internal/util"
	"strings"
)

type Prop struct {
	IsStatic bool // 是否是静态的
	Key, Val string
}

type Style struct {
	Key, Val string
}

type Attribute struct {
	Key, Val string
}

type Props []*Prop

type Styles map[string]string

func (p Styles) ToAttr() string {
	sortedKeys := util.GetSortedKey(map[string]string(p))
	var st strings.Builder
	for _, k := range sortedKeys {
		v := p[k]
		if st.Len() != 0 {
			st.WriteByte(' ')
		}
		st.WriteString(k + ": " + v + ";")
	}
	return fmt.Sprintf(`style="%s"`, st.String())
}

func (p Styles) Add(key string, val string) {
	p[key] = val
	return
}

func (p Styles) Merge(a Styles) {
	for k, v := range a {
		p[k] = v
	}
	return
}

func (p Styles) Del(key string) {
	delete(p, key)
}

type Class []string

func (c Class) ToAttr() string {
	return `class="` + strings.Join(c, " ") + `"`
}

func (c *Class) Remove(i string) {
	for index, k := range *c {
		if k == i {
			*c = append((*c)[:index], (*c)[index+1:]...)
			break
		}
	}
}

func (c *Class) Merge(a Class) {
	for _, v := range a {
		if c.Exist(v) {
			continue
		}

		*c = append(*c, v)
	}
}
func (c Class) Exist(a string) bool {
	for _, v := range c {
		if a == v {
			return true
		}
	}
	return false
}

func NewClass() Class {
	return make(Class, 0, 4)
}

type Attributes []Attribute
type Directives []Directive

func (p Props) Get(key string) (val string, exist bool) {
	for _, v := range p {
		if v.Key == key {
			return v.Val, true
		}
	}
	return
}

// 是否全部是静态Prop
func (p Props) IsStatic() bool {
	if len(p) == 0 {
		return true
	}

	for _, item := range p {
		if !item.IsStatic {
			return false
		}
	}

	return true
}
func (p *Props) Del(key string) {
	for index, k := range *p {
		if k.Key == key {
			*p = append((*p)[:index], (*p)[index+1:]...)
			break
		}
	}
}

type Directive struct {
	Name  string // v-animate
	Value string // {'a': 1}
	Arg   string // v-set:arg
}

type ElseIf struct {
	Types      string // else / elseif
	Condition  string // elseif语句的condition表达式
	VueElement *VueElement
}

type VIf struct {
	Condition string // 条件表达式
	// 当此节点是if节点是, 将与if指令匹配的elseif/else节点关联在一起
	ElseIf []*ElseIf
}

func (p *VIf) AddElseIf(v *ElseIf) {
	p.ElseIf = append(p.ElseIf, v)
}

type VFor struct {
	ArrayKey string
	ItemKey  string
	IndexKey string
}

type VSlot struct {
	SlotName string
	PropsKey string
}
type VBind struct {
	Val string
}

type VueElement struct {
	NodeType NodeType
	Tag      string
	Text     string

	PropClass  *Prop // 动态Class
	PropStyle  *Prop // 动态style
	Props      Props // props, 动态和静态, 不包括class和style
	VBind      *VBind
	Directives Directives   // 自定义指令, 运行时
	Class      Class         // 静态class
	Style      Styles        // 静态style
	Children   []*VueElement // 子节点
	VIf        *VIf          // 处理v-if需要的数据
	VFor       *VFor
	VSlot      *VSlot
	VElse      bool // 如果是VElse节点则不会生成代码(而是在vif里生成代码)
	VElseIf    bool
	// v-html / v-text
	// 支持v-html / v-text指令覆盖子级内容的组件有: template / html基本标签
	// component/slot和自定义组件不支持(没有必要)v-html/v-text覆盖子级
	VHtml string
	VText string
}

type VueElementParser struct {
}

func (p VueElementParser) Parse(e *Node) (*VueElement, error) {
	vs, err := p.parseList([]*Node{e})
	if err != nil {
		return nil, err
	}
	return vs[0], nil
}

// 递归处理同级节点
// 使用数组有一个好处就是方便的处理串联的v-if
func (p VueElementParser) parseList(es []*Node) (ve []*VueElement, err error) {
	vs := make([]*VueElement, 0)

	var ifVueEle *VueElement
	for _, e := range es {
		var props Props
		var propClass *Prop
		var propStyle *Prop
		var vBind *VBind
		var ds Directives
		var class []string
		var styles = Styles{}

		var vIf *VIf
		var vFor *VFor
		var vSlot *VSlot
		var vElse *ElseIf
		var vElseIf *ElseIf

		// v-html与v-text表达式
		var vHtml string
		var vText string

		for _, attr := range e.Attrs {
			oriKey := attr.Key
			ss := strings.Split(oriKey, ":")
			nameSpace := "-"
			key := oriKey
			if len(ss) == 2 {
				key = ss[1]
				nameSpace = ss[0]
			}

			if nameSpace == "v-bind" || nameSpace == "" {
				// v-bind:abc & :abc

				// 单独处理class和style
				if key == "class" {
					propClass = &Prop{
						Key: key,
						Val: attr.Value,
					}
				} else if key == "style" {
					propStyle = &Prop{
						Key: key,
						Val: attr.Value,
					}

				} else {
					props = append(props, &Prop{
						Key: key,
						Val: attr.Value,
					})
				}
			} else if strings.HasPrefix(oriKey, "v-") {
				// 指令
				// v-bind=""
				// v-if=""
				// v-slot:name=""
				// v-else-if=""
				// v-else
				// v-html
				// v-text
				// 自定义
				switch {
				case key == "v-bind":
					vBind = &VBind{
						Val: attr.Value,
					}
				case key == "v-for":
					val := attr.Value

					ss := strings.Split(val, " in ")
					arrayKey := strings.Trim(ss[1], " ")

					left := strings.Trim(ss[0], " ")
					var itemKey string
					var indexKey string
					// (item, index) in list
					if strings.Contains(left, ",") {
						left = strings.Trim(left, "()")
						ss := strings.Split(left, ",")
						itemKey = strings.Trim(ss[0], " ")
						indexKey = strings.Trim(ss[1], " ")
					} else {
						// (item) or item
						left = strings.Trim(left, "()")
						itemKey = left
						indexKey = "$index"
					}

					vFor = &VFor{
						ArrayKey: arrayKey,
						ItemKey:  itemKey,
						IndexKey: indexKey,
					}
				case key == "v-if":
					vIf = &VIf{
						Condition: strings.Trim(attr.Value, " "),
						ElseIf:    nil,
					}
				case nameSpace == "v-slot":
					slotName := key
					propsKey := attr.Value
					// 不应该为空, 否则可能会导致生成的go代码有误
					if propsKey == "" {
						propsKey = "slotProps"
					}
					vSlot = &VSlot{
						SlotName: slotName,
						PropsKey: propsKey,
					}
				case key == "v-else-if":
					vElseIf = &ElseIf{
						Types:     "elseif",
						Condition: strings.Trim(attr.Value, " "),
					}
				case key == "v-else":
					vElse = &ElseIf{
						Types:     "else",
						Condition: strings.Trim(attr.Value, " "),
					}
				case key == "v-html":
					vHtml = strings.Trim(attr.Value, " ")
				case key == "v-text":
					vText = strings.Trim(attr.Value, " ")
				default:
					// 自定义指令
					var name string
					var arg string
					if nameSpace != "-" {
						name = nameSpace
						arg = key
					} else {
						name = key
					}
					ds = append(ds, Directive{
						Name:  name,
						Value: strings.Trim(attr.Value, " "),
						Arg:   arg,
					})
				}
			} else if key == "class" {
				ss := strings.Split(attr.Value, " ")
				for _, v := range ss {
					if v != "" {
						class = append(class, v)
					}
				}
			} else if key == "style" {
				ss := strings.Split(attr.Value, ";")
				for _, v := range ss {
					v = strings.Trim(v, " ")
					ss := strings.Split(v, ":")
					if len(ss) != 2 {
						continue
					}
					key := strings.Trim(ss[0], " ")
					val := strings.Trim(ss[1], " ")

					styles.Add(key, val)
				}
			} else {
				props = append(props, &Prop{
					Key:      key,
					Val:      attr.Value,
					IsStatic: true,
				})
			}
		}

		ch, er := p.parseList(e.Child)
		if er != nil {
			err = er
			return
		}

		v := &VueElement{
			NodeType:   e.NodeType,
			Tag:        e.Tag,
			Text:       e.Text,
			PropClass:  propClass,
			PropStyle:  propStyle,
			Props:      props,
			Directives: ds,
			Class:      class,
			Style:      styles,
			Children:   ch,
			VIf:        vIf,
			VFor:       vFor,
			VSlot:      vSlot,
			VElse:      vElse != nil,
			VElseIf:    vElseIf != nil,
			VHtml:      vHtml,
			VText:      vText,
			VBind:      vBind,
		}

		// 记录vif, 接下来的elseif将与这个节点关联
		if vIf != nil {
			ifVueEle = v
		} else {
			// 如果有vif环境了, 但是中间跳过了, 则需要取消掉vif环境 (v-else 必须与v-if 相邻)
			skipNode := e.NodeType == CommentNode
			if !skipNode && vElse == nil && vElseIf == nil {
				ifVueEle = nil
			}
		}

		if vElseIf != nil {
			if ifVueEle == nil {
				err = errors.New("v-else-if must below v-if")
				return
			}
			vElseIf.VueElement = v
			ifVueEle.VIf.AddElseIf(vElseIf)

			// else 节点会被包括到if节点中, 不再放在当前节点中
			continue
		}
		if vElse != nil {
			if ifVueEle == nil {
				err = errors.New("v-else must below v-if")
				return
			}
			vElse.VueElement = v
			ifVueEle.VIf.AddElseIf(vElse)
			ifVueEle = nil
			continue
		}

		vs = append(vs, v)
	}

	return vs, nil
}

// 将html节点转换为Vue节点
func ToVueNode(node *Node) (vn *VueElement, err error) {
	return VueElementParser{}.Parse(node)
}

func (p *VueElement) NicePrint(showChild bool, lev int) string {
	s := strings.Repeat(" ", lev)
	switch p.NodeType {
	case ElementNode:
		s += fmt.Sprintf("<%v %+v", p.Tag, p.Props)
		if p.VIf != nil {
			s += " v-if=" + p.VIf.Condition
		}
		if p.VFor != nil {
			s += " v-for=" + p.VFor.ItemKey
		}
		if len(p.Props) != 0 {
			s += fmt.Sprintf(" Props: %+v", p.Props)
		}

		s += ">\n"
		if showChild {
			for _, v := range p.Children {
				s += fmt.Sprintf("%s", v.NicePrint(showChild, lev+1))
			}
		}

	case TextNode:
		s += fmt.Sprintf("%s\n", p.Text)
	case CommentNode:
		s += fmt.Sprintf("%s\n", p.Text)
	case DoctypeNode:
		s += fmt.Sprintf("%s\n", p.Text)
	}

	return s
}
