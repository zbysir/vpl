package parser

import (
	"errors"
	"fmt"
	"github.com/zbysir/vpl/internal/util"
	"strings"
)

type Prop struct {
	IsStatic  bool // 是否是静态的(v-bind:语法是动态, 之外是静态)
	CanBeAttr bool // 是否当成attr输出
	Key       string
	StaticVal interface{} // 静态的value, 如style和class在编译时就会被解析成map和slice
	ValCode   string      // 如果props是动态的, valCode存储js表达式
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
	return st.String()
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
	return strings.Join(c, " ")
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
			return v.ValCode, true
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
	Name  string // animate
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
	// 是否分配调用组件时传递来的属性.
	// 如果组件中只存在一个root节点, 则此节点会自动分配属性. 否则所有root节点都不会.
	// (fragments: https://v3.vuejs.org/guide/migration/fragments.html#overview)
	DistributionAttr bool

	//PropClass  *Prop // 动态Class
	//PropStyle  *Prop // 动态style
	Props      Props // props, 动态和静态, 包括class和style
	VBind      *VBind
	Directives Directives // 自定义指令, 运行时
	//Class      Class         // 静态class
	//Style      Styles        // 静态style
	Children []*VueElement // 子节点
	VIf      *VIf          // 处理v-if需要的数据
	VFor     *VFor
	VSlot    *VSlot
	VElse    bool // 如果是VElse节点则不会生成代码(而是在vif里生成代码)
	VElseIf  bool
	// v-html / v-text
	// 支持v-html / v-text指令覆盖子级内容的组件有: template / html基本标签
	// component/slot和自定义组件不支持(没有必要)v-html/v-text覆盖子级
	VHtml string
	VText string
}

type ParseVueNodeOptions struct {
	CanBeAttr   func(k string) bool
	SkipComment bool
}

type VueElementParser struct {
	options *ParseVueNodeOptions
}

func (p VueElementParser) Parse(e *Node) (*VueElement, error) {
	vs, err := p.parseList([]*Node{e})
	if err != nil {
		return nil, err
	}

	ve := vs[0]

	// 如果根节点只有要给并且是template，则是vue写法, 需要删除掉template来兼容此语法
	if len(ve.Children) == 1 && ve.Children[0].Tag == "template" {
		ve.Children = ve.Children[0].Children
	}

	// 如果只有一个root节点, 则将自动分配attr
	var childLen int64
	for _, c := range ve.Children {
		if c.NodeType != CommentNode {
			childLen++
		}
	}

	if childLen == 1 {
		for _, c := range ve.Children {
			if c.NodeType == ElementNode {
				c.DistributionAttr = true

				// 如果if节点需要分发attr，那么else节点也需要
				if c.VIf != nil {
					for _, e := range c.VIf.ElseIf {
						e.VueElement.DistributionAttr = true
					}
				}
			}

		}
	}

	return ve, nil
}

// 递归处理同级节点
// 使用数组有一个好处就是方便的处理串联的v-if
func (p VueElementParser) parseList(es []*Node) (ve []*VueElement, err error) {
	vs := make([]*VueElement, 0)

	var ifVueEle *VueElement
	for _, e := range es {
		if p.options.SkipComment {
			if e.NodeType == CommentNode {
				continue
			}
		}

		var props Props
		//var propClass *Prop
		//var propStyle *Prop
		var vBind *VBind
		var ds Directives
		//var class []string
		//var styles = Styles{}

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
					props = append(props, &Prop{
						IsStatic:  false,
						CanBeAttr: true,
						Key:       "class",
						ValCode:   attr.Value,
					})
				} else if key == "style" {
					props = append(props, &Prop{
						IsStatic:  false,
						CanBeAttr: true,
						Key:       "style",
						ValCode:   attr.Value,
					})
				} else {
					// 动态prosp
					props = append(props, &Prop{
						IsStatic:  false,
						CanBeAttr: p.options.CanBeAttr(key),
						Key:       key,
						ValCode:   attr.Value,
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
						Name:  strings.TrimPrefix(name, "v-"),
						Value: strings.Trim(attr.Value, " "),
						Arg:   arg,
					})
				}
			} else if strings.HasPrefix(key, "#") {
				// v-slot:缩小
				slotName := key[1:]
				propsKey := attr.Value
				vSlot = &VSlot{
					SlotName: slotName,
					PropsKey: propsKey,
				}
			} else if key == "class" {
				ss := strings.Split(attr.Value, " ")
				// class的基础类型是[]interface. 和js表达式运行之后的结果类型保持一致.
				var staticVal []interface{}
				for _, v := range ss {
					if v != "" {
						staticVal = append(staticVal, v)
					}
				}
				props = append(props, &Prop{
					IsStatic:  true,
					CanBeAttr: true,
					Key:       "class",
					//Val:       attr.Value,
					StaticVal: staticVal,
				})

			} else if key == "style" {
				ss := strings.Split(attr.Value, ";")
				// style的基础类型是map[string]interface{}, 和js表达式运行之后的结果类型保持一致.
				staticVal := map[string]interface{}{}

				for _, v := range ss {
					v = strings.Trim(v, " ")
					ss := strings.Split(v, ":")
					if len(ss) != 2 {
						continue
					}
					key := strings.Trim(ss[0], " ")
					val := strings.Trim(ss[1], " ")

					staticVal[key] = val
				}

				props = append(props, &Prop{
					IsStatic:  true,
					CanBeAttr: true,
					Key:       "style",
					//Val:       attr.Value,
					StaticVal: staticVal,
				})
			} else {
				// 静态props
				props = append(props, &Prop{
					CanBeAttr: p.options.CanBeAttr(key),
					Key:       key,
					//Val:       attr.Value,
					StaticVal: attr.Value,
					IsStatic:  true,
				})
			}
		}

		ch, er := p.parseList(e.Child)
		if er != nil {
			err = er
			return
		}

		v := &VueElement{
			NodeType: e.NodeType,
			Tag:      e.Tag,
			Text:     e.Text,
			//PropClass:  propClass,
			//PropStyle:  propStyle,
			Props:      props,
			Directives: ds,
			//Class:      class,
			//Style:      styles,
			Children: ch,
			VIf:      vIf,
			VFor:     vFor,
			VSlot:    vSlot,
			VElse:    vElse != nil,
			VElseIf:  vElseIf != nil,
			VHtml:    vHtml,
			VText:    vText,
			VBind:    vBind,
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
func ToVueNode(node *Node, options *ParseVueNodeOptions) (vn *VueElement, err error) {
	if options == nil {
		options = &ParseVueNodeOptions{
			CanBeAttr: func(k string) bool {
				if k == "id" || k == "class" || k == "style" {
					return true
				}

				if strings.HasPrefix(k, "data-") {
					return true
				}

				return false
			},
			SkipComment: false,
		}
	}
	return VueElementParser{
		options: options,
	}.Parse(node)
}

func (p *VueElement) NicePrint(showChild bool, lev int) string {
	s := strings.Repeat(" ", lev)
	index := strings.Repeat(" ", lev)
	switch p.NodeType {
	case ElementNode, RootNode:
		s += fmt.Sprintf("<%v", p.Tag)
		if p.VIf != nil {
			s += " v-if=" + p.VIf.Condition
		}
		if p.VFor != nil {
			s += " v-for=" + p.VFor.ItemKey
		}
		if p.DistributionAttr {
			s += " DistributionAttr"
		}
		if len(p.Props) != 0 {
			s += fmt.Sprintf(" Props: %s", nicePrintProps(p.Props))
		}

		s += ">\n"
		if showChild {
			for _, v := range p.Children {
				s += fmt.Sprintf("%s", v.NicePrint(showChild, lev+1))
			}
		}

		// velse
		if p.VIf != nil {
			for _, ef := range p.VIf.ElseIf {
				if ef.Condition != "" {
					s += fmt.Sprintf("%sElseIf(%+v)\n", index, ef.Condition)
				} else {
					s += fmt.Sprintf("%sElse\n", index)
				}

				s += fmt.Sprintf("%s", ef.VueElement.NicePrint(showChild, lev+1))
			}

		}
	case TextNode:
		s += fmt.Sprintf("%s\n", p.Text)
	case CommentNode:
		s += fmt.Sprintf("%s\n", p.Text)
	case DoctypeNode:
		s += fmt.Sprintf("%s\n", p.Text)
		//case RootNode:
		//	s += fmt.Sprintf("<ROOT>\n")
	}

	return s
}

func nicePrintProps(p Props) string {
	var s strings.Builder
	s.WriteString("[")
	for _, i := range p {
		s.WriteString(fmt.Sprintf("%+v, ", i))
	}
	s.WriteString("]")

	return s.String()
}
