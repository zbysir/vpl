package vpl

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/robertkrimen/otto/ast"
	"github.com/zbysir/vpl/internal/lib/log"
	"github.com/zbysir/vpl/internal/parser"
	"github.com/zbysir/vpl/internal/util"
	"strings"
	"sync"
)

// 执行每一个块的上下文
type Scope struct {
	Parent *Scope
	Value  map[string]interface{}
}

func (s *Scope) Get(k string) interface{} {
	return s.GetDeep(k)
}

func NewScope(outer *Scope) *Scope {
	return &Scope{
		Parent: outer,
		Value:  nil,
	}
}

// 获取作用域中的变量
// 会向上查找
func (s *Scope) GetDeep(k ...string) (v interface{}) {
	var rootExist bool
	var ok bool

	curr := s
	for curr != nil {
		v, rootExist, ok = ShouldLookInterface(curr.Value, k...)
		// 如果root存在, 则说明就应该读取当前作用域, 否则向上层作用域查找
		if rootExist {
			if !ok {
				return nil
			} else {
				return
			}
		}

		curr = curr.Parent
	}

	return
}

func (s *Scope) Extend(data map[string]interface{}) *Scope {
	return &Scope{
		Parent: s,
		Value:  data,
	}
}

// 设置暂时只支持在当前作用域设置变量
// 避免对上层变量造成副作用
func (s *Scope) Set(k string, v interface{}) {
	if s.Value == nil {
		s.Value = map[string]interface{}{}
	}
	(s.Value)[k] = v
}

// 在渲染中的上下文, 用在function和directive
type RenderCtx struct {
	Scope *Scope // 当前作用域, 用于向当前作用域声明一个值
	Store Store  // 用于共享数据, 此Store是RenderParam中传递的Store
}

type Directive func(ctx *RenderCtx, nodeData *NodeData, binding *DirectivesBinding)

type DirectivesBinding struct {
	Value interface{}
	Arg   string
	Name  string
}

// 编译之后的Prop
// 将js表达式解析成AST, 加速运行
type propC struct {
	CanBeAttr bool
	Key       string
	Val       expression
	IsStatic  bool
	ValStatic string // 如果Prop是静态的, 那么会在编译时优化为字符串
}

type propsC []*propC

// for nicePrint
func (r propsC) String() string {
	str := "["
	for _, v := range r {
		beAttr := ""
		if v.CanBeAttr {
			beAttr = "(attr)"
		}
		var val interface{} = v.Val

		if v.IsStatic {
			val = v.ValStatic
		}

		str += fmt.Sprintf("%+v%v: %+v, ", v.Key, beAttr, val)
	}
	str = strings.TrimSuffix(str, ", ")
	str += "]"
	return str
}

func (r propsC) execTo(ctx *RenderCtx, ps *Props) {
	if len(r) == 0 {
		return
	}

	for _, p := range r {
		c := CanNotBeAttr
		if p.CanBeAttr {
			c = CanBeAttr
		}

		ps.append(&PropKeys{
			AttrWay: c,
			Key:     p.Key,
		}, p.exec(ctx).Val)
	}
	return
}

func (r *propC) exec(ctx *RenderCtx) *Prop {
	if r == nil {
		return &Prop{}
	}
	if r.IsStatic {
		return &Prop{Key: r.Key, Val: r.ValStatic}
	} else {
		return &Prop{Key: r.Key, Val: r.Val.Exec(ctx)}
	}
}

// 数值Prop
// 执行PropC会得到PropR
type Prop struct {
	Key string
	Val interface{}
}

//type PropKey struct {
//	AttrWay AttrWay // 能否被当成Attr输出
//	Key     string
//}

type AttrWay uint8

const (
	MayBeAttr    AttrWay = 0 // 无法在编译时确定, 还需要在运行时判断
	CanBeAttr    AttrWay = 1 // 在编译时就确定能够当做attr
	CanNotBeAttr AttrWay = 2 // 在编译时就确定不能够当做attr
)

type PropKeys struct {
	AttrWay AttrWay // 能否被当成Attr输出
	Key     string
	Last    *PropKeys // 如果LinkKey只有一个元素, 则last是自己
	Next    *PropKeys
}

func (l *PropKeys) Append(a *PropKeys) {
	if l.Key == "" {
		l.Key = a.Key
		l.Last = l
		l.Next = nil
		l.AttrWay = a.AttrWay
		return
	}

	if l.Last == nil {
		l.Last = l
	}
	l.Last.Next = a
	l.Last = a
}

type Props struct {
	keys *PropKeys              // 在生成attr时会用到顺序
	data map[string]interface{} // 存储map有利于快速取值
}

func NewProps() *Props {
	return &Props{
		keys: nil,
		data: nil,
	}
}

func (r *Props) ForEach(cb func(index int, k *PropKeys, v interface{})) {
	if r == nil {
		return
	}
	i := 0
	h := r.keys
	for h != nil {
		cb(i, h, r.data[h.Key])
		h = h.Next
		i++
	}
	return
}

func (r *Props) ToMap() map[string]interface{} {
	if r == nil {
		return nil
	}
	return r.data
}

func (r *Props) append(k *PropKeys, v interface{}) {
	if r.data == nil {
		r.data = map[string]interface{}{}
	}
	if r.keys == nil {
		r.keys = &PropKeys{}
	}

	// 合并class/style
	ve, exist := r.data[k.Key]
	if !exist {
		r.keys.Append(k)
	} else {
		switch k.Key {
		case "class":
			v = margeClass(ve, v)
		case "style":
			v = margeStyle(ve, v)
		}
	}

	r.data[k.Key] = v
}

// AppendAttr 向当前tag添加属性
func (r *Props) AppendAttr(k, v string) {
	r.append(&PropKeys{
		AttrWay: CanBeAttr,
		Key:     k,
	}, v)
}

func (r *Props) Append(k string, v interface{}) {
	r.append(&PropKeys{
		AttrWay: MayBeAttr,
		Key:     k,
	}, v)
}

func (r *Props) AppendClass(c ...string) {
	cla := make([]interface{}, len(c))
	for i, v := range c {
		cla[i] = v
	}
	r.append(&PropKeys{
		AttrWay: CanBeAttr,
		Key:     "class",
	}, cla)
}

func (r *Props) AppendStyle(st map[string]string) {
	stm := make(map[string]interface{}, len(st))
	for k, v := range st {
		stm[k] = v
	}
	r.append(&PropKeys{
		AttrWay: CanBeAttr,
		Key:     "style",
	}, stm)
}

func margeClass(a interface{}, b interface{}) (d interface{}) {
	ar := []interface{}{a, b}
	return ar
}

func margeStyle(a interface{}, b interface{}) (d interface{}) {
	var ar map[string]interface{}
	if at, ok := a.(map[string]interface{}); ok {
		ar = at
	} else {
		ar = map[string]interface{}{}
	}

	if bt, ok := b.(map[string]interface{}); ok {
		for k, v := range bt {
			ar[k] = v
		}
	}

	return ar
}

// 无序添加多个props
func (r *Props) AppendMap(mp map[string]interface{}) {
	keys := util.GetSortedKey(mp)

	for _, k := range keys {
		v := mp[k]
		r.append(&PropKeys{
			AttrWay: MayBeAttr,
			Key:     k,
		}, v)
	}
}

// 有序添加多个props
func (r *Props) appendProps(ps *Props) {
	if ps == nil {
		return
	}

	ps.ForEach(func(index int, k *PropKeys, v interface{}) {
		r.append(k, v)
	})
}

func (r *Props) Get(key string) (interface{}, bool) {
	v, exist := r.data[key]
	return v, exist
}

// 如果 style和class动态与静态不冲突 ,并且沒有指令, 则可以将静态style/class优化为 string
func compileProps(p parser.Props, staticProp bool) (propsC, error) {
	pc := make(propsC, len(p))
	hasBindStyle := false
	hasBindClass := false
	for _, v := range p {
		if !v.IsStatic && v.Key == "class" {
			hasBindClass = true
		}
		if !v.IsStatic && v.Key == "style" {
			hasBindStyle = true
		}
	}
	for i, v := range p {
		static := staticProp
		if staticProp {
			if v.Key == "class" && v.IsStatic && hasBindClass {
				static = false
			}
			if v.Key == "style" && v.IsStatic && hasBindStyle {
				static = false
			}
		}

		p, err := compileProp(v, static)
		if err != nil {
			return nil, err
		}
		pc[i] = p
	}
	return pc, nil
}

func compileProp(p *parser.Prop, staticProp bool) (*propC, error) {
	if p == nil {
		return nil, nil
	}

	pc := &propC{
		Key:       p.Key,
		CanBeAttr: p.CanBeAttr,
	}
	if p.IsStatic {
		// 如果是静态的, 并且需要优化为字符串, 则修改为字符串
		if staticProp {
			switch p.Key {
			case "style":
				pc.ValStatic = getStyleFromProps(p.StaticVal).ToAttr()
			case "class":
				pc.ValStatic = getClassFromProps(p.StaticVal).ToAttr()
			default:
				pc.ValStatic = p.StaticVal.(string)
			}
			pc.IsStatic = true
		} else {
			pc.Val = newRawExpression(p.StaticVal)
		}
	} else {
		if p.ValCode != "" {
			node, err := compileJS(p.ValCode)
			if err != nil {
				return nil, fmt.Errorf("parseJs err: %w", err)
			}
			pc.Val = &jsExpression{node: node, code: p.ValCode}
		} else {
			pc.Val = &nullExpression{}
		}
	}

	return pc, nil
}

func compileVBind(v *parser.VBind) (*vBindC, error) {
	if v == nil {
		return nil, nil
	}

	if v.Val == "" {
		return nil, nil
	}

	node, err := compileJS(v.Val)
	if err != nil {
		return nil, fmt.Errorf("parseJs err: %w", err)
	}

	return &vBindC{val: &jsExpression{node: node, code: v.Val}}, nil
}

func compileDirective(ds parser.Directives) (directivesC, error) {
	if len(ds) == 0 {
		return nil, nil
	}

	pc := make(directivesC, len(ds))
	for i, v := range ds {
		node, err := compileJS(v.Value)
		if err != nil {
			return nil, fmt.Errorf("parseJs err: %w", err)
		}
		pc[i] = directiveC{
			Name:  v.Name,
			Value: &jsExpression{node: node, code: v.Value},
			Arg:   v.Arg,
		}
	}
	return pc, nil
}

// 作用在tag的所有属性
type tagStruct struct {
	// Props: 无论动态还是静态, 都是Props(包括class与style, 这是为了实现v-bind='$props'语法).
	// 静态的attr也处理成Props是为了保持顺序, 当然也是为了减少概念
	//
	//  如: <div id="abc" :data-id="id" :style="{left: '1px'}">
	//  其中 Props 值为: id=string, data-id=string, style=map[string]interface
	//
	// 另外tag上的 Props 会根据CanBeAttrKey设置被转为html attr.
	Props propsC
	VBind *vBindC

	Directives directivesC
	Slots      *SlotsC
}

// 编译时的指令
type directiveC struct {
	Name  string     // v-animate
	Value expression // {'a': 1}
	Arg   string     // v-set:arg
}

type directivesC []directiveC

// 组件的属性
type ComponentStruct struct {
	// Props: 无论动态还是静态, 都是Props
	//
	//  如: <Menu :data="data" id="abc" :style="{left: '1px'}">
	//  其中 Props 值为: data, id
	//  其中 PropStyle 值为: style
	Props propsC
	// PropClass指动态class
	//  如 <Menu :class="['a','b']" class="c">
	//  那么PropClass的值是: ['a', 'b']
	VBind *vBindC

	Directives directivesC
	// 传递给这个组件的Slots
	Slots *SlotsC
}

// VBind 语法, 一次传递多个prop
// v-bind='{id: id, 'other-attr': otherAttr}'
// 有一个特殊用法:
//  v-bind='$props': 将父组件所有的 props(不包括class和style) 一起传给子组件
type vBindC struct {
	useProps bool
	val      expression
}

func (v *vBindC) execTo(ctx *RenderCtx, ps *Props) {
	if v == nil {
		return
	}
	var b interface{}
	if v.useProps {
		b = ctx.Scope.Get("$props")
	} else {
		b = v.val.Exec(ctx)
	}
	switch t := b.(type) {
	case map[string]interface{}:
		ps.AppendMap(t)
	case skipMarshalMap:
		ps.AppendMap(t)
	case *Props:
		ps.appendProps(t)
	default:
		panic(fmt.Sprintf("bad Type of Vbind: %T", b))
	}
}

func (v *vBindC) exec(ctx *RenderCtx) interface{} {
	if v == nil {
		return nil
	}
	if v.useProps {
		return ctx.Scope.Get("$props")
	} else {
		return v.val.Exec(ctx)
	}

}

// 表达式, 所有js表达式都会被预编译成为expression
type expression interface {
	// 根据scope计算表达式值
	Exec(ctx *RenderCtx) interface{}
}

// 原始值
type rawExpression struct {
	raw interface{}
}

func (r *rawExpression) String() string {
	return fmt.Sprintf("%v", r.raw)
}

func (r *rawExpression) Exec(*RenderCtx) interface{} {
	return r.raw
}

func newRawExpression(raw interface{}) *rawExpression {
	return &rawExpression{raw: raw}
}

type jsExpression struct {
	node ast.Node
	code string
}

func (r *jsExpression) Exec(ctx *RenderCtx) interface{} {
	v, err := runJsExpression(r.node, ctx)
	if err != nil {
		log.Warningf("runJsExpression err:%v", err)
		return err
	}

	return v
}

func (r *jsExpression) String() string {
	return r.code
}

type nullExpression struct {
}

func (r *nullExpression) Exec(*RenderCtx) interface{} {
	return nil
}

// vue语法会被编译成一组Statement
// 为了避免多次运行造成副作用, 所有的 运行时代码 都不应该修改 编译时
type Statement interface {
	Exec(ctx *StatementCtx, o *StatementOptions) error
}

type FuncStatement func(*StatementCtx, *StatementOptions) error

func (f FuncStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	return f(ctx, o)
}

type Writer interface {
	// 如果需要实现异步计算, 则需要将span存储, 在最后统一计算出string.
	WriteSpan(Span)
	// 如果是同步计算, 使用WriteString会将string结果直接存储或者拼接
	WriteString(string)
	Result() string
}

type Span interface {
	Result() string
}

// 静态字符串块
type StrStatement struct {
	Str string
}

func (s *StrStatement) Exec(ctx *StatementCtx, _ *StatementOptions) error {
	ctx.W.WriteString(s.Str)
	return nil
}

type EmptyStatement struct {
}

func (s *EmptyStatement) Exec(_ *StatementCtx, _ *StatementOptions) error {
	return nil
}

// tag块
type tagStatement struct {
	tag       string
	tagStruct tagStruct
}

// 执行map格式的props(来至v-bind语法)
func execBindProps(t map[string]interface{}, ctx *StatementCtx, attrKeys *[]string, attr *map[string]string, class *strings.Builder, style *map[string]interface{}) {
	keys := util.GetSortedKey(t)

	for _, k := range keys {
		v := t[k]
		if k == "class" {
			writeClass(v, class)
			if _, exist := (*attr)["class"]; !exist {
				*attrKeys = append(*attrKeys, "class")
				(*attr)["class"] = ""
			}
		} else if k == "style" {
			switch t := v.(type) {
			case map[string]interface{}:
				if *style == nil {
					*style = t
				} else {
					for k, v := range t {
						(*style)[k] = v
					}
				}
			}
			if _, exist := (*attr)["style"]; !exist {
				*attrKeys = append(*attrKeys, "style")
				(*attr)["style"] = ""
			}
		} else {
			if ctx.CanBeAttrsKey(k) {
				if _, exist := (*attr)[k]; !exist {
					*attrKeys = append(*attrKeys, k)
				}

				switch v := v.(type) {
				case string:
					(*attr)[k] = v
				default:
					(*attr)[k] = util.InterfaceToStr(v, true)
				}
			}
		}
	}
}

// 如果tag没有指令, 则也不需要生成props, 而是将propsC直接运行成为attr.
func (t *tagStatement) ExecAttr(ctx *StatementCtx, rCtx *RenderCtx) error {
	var style map[string]interface{}
	var class strings.Builder

	// 使用attr来解决attr会合并的问题.
	// 如组件外传递的attr会覆盖与根组件相同的attr
	attrKeys := make([]string, 0, len(t.tagStruct.Props))
	attr := make(map[string]string, len(t.tagStruct.Props))

	if len(t.tagStruct.Props) != 0 {
		for _, p := range t.tagStruct.Props {
			if p.Key == "class" {
				// 如果class是静态的, 则不会发生合并的情况, 直接写入到write.
				if p.IsStatic {
					ctx.W.WriteString(` class="`)
					ctx.W.WriteString(p.ValStatic)
					ctx.W.WriteString(`"`)
				} else {
					writeClass(p.Val.Exec(rCtx), &class)
					if _, exist := attr["class"]; !exist {
						attrKeys = append(attrKeys, "class")
						attr["class"] = ""
					}
				}
			} else if p.Key == "style" {
				if p.IsStatic {
					ctx.W.WriteString(` style="`)
					ctx.W.WriteString(p.ValStatic)
					ctx.W.WriteString(`"`)
				} else {
					switch t := p.Val.Exec(rCtx).(type) {
					case map[string]interface{}:
						if style == nil {
							style = t
						} else {
							for k, v := range t {
								style[k] = v
							}
						}
					}
					if _, exist := attr["style"]; !exist {
						attrKeys = append(attrKeys, "style")
						attr["style"] = ""
					}
				}
			} else {
				if p.IsStatic {
					if p.ValStatic != "" {
						if _, exist := attr[p.Key]; !exist {
							attrKeys = append(attrKeys, p.Key)
						}
						attr[p.Key] = p.ValStatic
					}
				} else {
					v := p.Val.Exec(rCtx)
					if _, exist := attr[p.Key]; !exist {
						attrKeys = append(attrKeys, p.Key)
					}

					switch v := v.(type) {
					case string:
						attr[p.Key] = v
					default:
						attr[p.Key] = util.InterfaceToStr(v, true)
					}
				}
			}
		}
	}

	// 可能需要将 props和vBind中重复的attr去重
	if t.tagStruct.VBind != nil {
		var b = t.tagStruct.VBind.exec(rCtx)
		switch t := b.(type) {
		case nil:
		case map[string]interface{}:
			execBindProps(t, ctx, &attrKeys, &attr, &class, &style)
		case skipMarshalMap:
			execBindProps(t, ctx, &attrKeys, &attr, &class, &style)
		default:
			panic(fmt.Sprintf("bad Type of Vbind: %T", b))
		}
	}

	// 保证attr排序和写的一致
	// style和class会出现合并的情况, 所以在运行完所有props和vBind之后处理.
	for i := range attrKeys {
		switch attrKeys[i] {
		case "style":
			ctx.W.WriteString(` style="`)
			sortedKeys := util.GetSortedKey(style)
			var st strings.Builder
			for _, k := range sortedKeys {
				v := style[k]
				if st.Len() != 0 {
					st.WriteByte(' ')
				}

				st.WriteString(k)
				st.WriteString(": ")
				switch v := v.(type) {
				case string:
					st.WriteString(util.Escape(v))
				default:
					bs, _ := json.Marshal(v)
					st.WriteString(util.Escape(string(bs)))
				}
				st.WriteByte(';')
			}
			ctx.W.WriteString(st.String())
			ctx.W.WriteString(`"`)
		case "class":
			ctx.W.WriteString(` class="`)
			ctx.W.WriteString(class.String())
			ctx.W.WriteString(`"`)
		default:
			ctx.W.WriteString(` `)
			ctx.W.WriteString(attrKeys[i])
			v := attr[attrKeys[i]]
			if v != "" {
				ctx.W.WriteString(`="`)
				ctx.W.WriteString(v)
				ctx.W.WriteString(`"`)
			}
		}
	}

	return nil
}

func (t *tagStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	rCtx := ctxPool.Get().(*RenderCtx)
	rCtx.Store = ctx.Store
	rCtx.Scope = o.Scope
	defer ctxPool.Put(rCtx)

	ctx.W.WriteString("<" + t.tag)

	// 如果没有指令, 则优化props执行流程
	// - 只执行CanBeAttr的props
	// - 优化class与style
	// - 直接写入Writer, 减少props声明

	// 如果没有指令, 则不需要闭包slot作用域
	var slots *Slots

	if len(t.tagStruct.Directives) != 0 {
		// 处理attr
		// 计算Props
		var props *Props
		if len(t.tagStruct.Props) != 0 || t.tagStruct.VBind != nil {
			props = NewProps()

			if len(t.tagStruct.Props) != 0 {
				t.tagStruct.Props.execTo(rCtx, props)
			}

			// v-bind="{id: 1}" 语法, 将计算出整个PropsR
			if t.tagStruct.VBind != nil {
				t.tagStruct.VBind.execTo(rCtx, props)
			}
		}

		// 只有指令有修改slots的需求, 如果没有指令, 则不需要闭包slot作用域
		slots = t.tagStruct.Slots.WrapScope(o)

		// 执行指令
		// 指令可以修改scope/props/style/class/children
		data := &NodeData{
			Props: props,
			Slots: slots,
		}
		execDirectives(t.tagStruct.Directives, ctx, o.Scope, data)
		props = data.Props
		slots = data.Slots

		if props != nil {
			props.ForEach(func(index int, k *PropKeys, v interface{}) {
				// 如果在编译期就确定了不能被转为attr, 则始终不能
				// 如果无法在编译期间确定(如 通过props.AppendMap()的方式添加的props/通过v-bind="$props"方式而来的props), 则还需要再次调用函数判断
				if k.AttrWay == CanNotBeAttr {
					return
				}
				if k.AttrWay == MayBeAttr {
					// style和class字段始终会作为attr
					if k.Key != "style" && k.Key != "class" {
						if !ctx.CanBeAttrsKey(k.Key) {
							return
						}
					}
				}

				if k.Key == "style" {
					if v != nil {
						ctx.W.WriteString(` style="`)
						var s strings.Builder
						writeStyle(v, &s)
						ctx.W.WriteString(s.String())
						ctx.W.WriteString(`"`)
					}
				} else if k.Key == "class" {
					if v != nil {
						ctx.W.WriteString(` class="`)
						var s strings.Builder
						writeClass(v, &s)
						ctx.W.WriteString(s.String())
						ctx.W.WriteString(`"`)
					}
				} else {
					ctx.W.WriteString(" ")
					ctx.W.WriteString(k.Key)

					if v != nil {
						ctx.W.WriteString(`="`)

						switch v := v.(type) {
						case string:
							ctx.W.WriteString(v)
						default:
							ctx.W.WriteString(util.InterfaceToStr(v, true))
						}
						ctx.W.WriteString(`"`)
					}
				}

			})
		}

	} else {
		err := t.ExecAttr(ctx, rCtx)
		if err != nil {
			return err
		}
	}

	ctx.W.WriteString(">")

	// 如果没有指令, 则不需要闭包slot作用域
	if slots != nil {
		// 子节点
		children := slots.Default
		if children != nil {
			err := children.ExecSlot(ctx, nil)
			if err != nil {
				return err
			}
		}
	} else if t.tagStruct.Slots != nil {
		// 直接执行children, 而不当做slots
		children := t.tagStruct.Slots.Default
		if children != nil && children.Children != nil {
			err := children.Children.Exec(ctx, o)
			if err != nil {
				return err
			}
		}
	}

	ctx.W.WriteString("</" + t.tag + ">")
	return nil
}

func execDirectives(ds directivesC, ctx *StatementCtx, scope *Scope, o *NodeData) {
	rCtx := ctxPool.Get().(*RenderCtx)
	rCtx.Store = ctx.Store
	rCtx.Scope = scope
	defer ctxPool.Put(rCtx)

	for _, v := range ds {
		val := v.Value.Exec(rCtx)
		d, exist := ctx.Directives[v.Name]
		if exist {
			d(
				rCtx,
				o,
				&DirectivesBinding{
					Value: val,
					Arg:   v.Arg,
					Name:  v.Name,
				},
			)
		}

	}
}

type Class = parser.Class
type Styles = parser.Styles

type NodeData struct {
	Props *Props // 给组件添加attr
	Slots *Slots
}

// 支持的格式: map[string]interface{}
func getStyleFromProps(styleProps interface{}) Styles {
	if styleProps == nil {
		return Styles{}
	}
	st := Styles{}
	switch t := styleProps.(type) {
	case map[string]interface{}:
		for k, v := range t {
			switch v := v.(type) {
			case string:
				st.Add(k, util.Escape(v))
			default:
				bs, _ := json.Marshal(v)
				st.Add(k, util.Escape(string(bs)))
			}
		}
	}

	return st
}

// 支持的格式: map[string]interface{}
func writeStyle(styleProps interface{}, w *strings.Builder) {
	if styleProps == nil {
		return
	}

	switch t := styleProps.(type) {
	case map[string]interface{}:
		sortedKeys := util.GetSortedKey(t)
		for _, k := range sortedKeys {
			v := t[k]
			if w.Len() != 0 {
				w.WriteByte(' ')
			}
			w.WriteString(k + ": ")

			switch v := v.(type) {
			case string:
				w.WriteString(util.Escape(v))
			default:
				bs, _ := json.Marshal(v)
				w.WriteString(util.Escape(string(bs)))
			}

			w.WriteString(";")
		}
	}

	return
}

type ifStatement struct {
	conditionCode  string
	condition      expression
	ChildStatement Statement
	ElseIf         []*elseStatement
}

type elseStatement struct {
	conditionCode string
	// 如果condition为空 则说明是else节点, 否则是elseif节点
	condition      expression
	ChildStatement Statement
}

func (i *ifStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	rCtx := ctxPool.Get().(*RenderCtx)
	rCtx.Store = ctx.Store
	rCtx.Scope = o.Scope
	defer ctxPool.Put(rCtx)
	r := i.condition.Exec(rCtx)
	if util.InterfaceToBool(r) {
		err := i.ChildStatement.Exec(ctx, o)
		if err != nil {
			return err
		}
	} else {
		// 如果if没有判断成功, 则循环执行elseIf
		for _, ef := range i.ElseIf {
			// 如果condition为空 则说明是else节点
			if ef.condition == nil {
				err := ef.ChildStatement.Exec(ctx, o)
				if err != nil {
					return err
				}
				break
			}
			if util.InterfaceToBool(ef.condition.Exec(rCtx)) {
				err := ef.ChildStatement.Exec(ctx, o)
				if err != nil {
					return err
				}
				break
			}
		}
	}

	return nil
}

type forStatement struct {
	ArrayKey    string
	Array       expression
	ItemKey     string
	IndexKey    string
	ChildChunks Statement
}

func (f forStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	rCtx := ctxPool.Get().(*RenderCtx)
	rCtx.Store = ctx.Store
	rCtx.Scope = o.Scope
	array := f.Array.Exec(rCtx)
	ctxPool.Put(rCtx)

	return util.ForInterface(array, func(index int, v interface{}) error {
		scope := o.Scope.Extend(map[string]interface{}{
			f.IndexKey: index,
			f.ItemKey:  v,
		})

		err := f.ChildChunks.Exec(ctx, &StatementOptions{
			Scope: scope,
		})
		if err != nil {
			return err
		}

		return nil
	})
}

type groupStatement struct {
	s         []Statement
	strBuffer strings.Builder
}

// 调用GroupStatement.Append之后还必须调用Finish才能保证GroupStatement中的数据是正确的
func (g *groupStatement) Finish() Statement {
	if g.strBuffer.Len() != 0 {
		g.s = append(g.s, &StrStatement{Str: g.strBuffer.String()})
		g.strBuffer.Reset()
	}

	if len(g.s) == 0 {
		return &EmptyStatement{}
	}

	if len(g.s) == 1 {
		return g.s[0]
	}
	return g
}

func (g *groupStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	for i := range g.s {
		err := g.s[i].Exec(ctx, o)
		if err != nil {
			return err
		}
	}

	return nil
}

// Append 拼接一个新的语句到组里, 如果有连续的字符串语句 则会合并成为一个字符串语句.
func (g *groupStatement) Append(st Statement) {
	if st == nil {
		return
	}
	switch appT := st.(type) {
	case *EmptyStatement:
		return
	case *StrStatement:
		g.strBuffer.WriteString(appT.Str)
	case *groupStatement:
		for _, v := range appT.s {
			g.Append(v)
		}
	default:
		if g.strBuffer.Len() != 0 {
			g.s = append(g.s, &StrStatement{Str: g.strBuffer.String()})
			g.strBuffer.Reset()
		}
		g.s = append(g.s, st)
	}
}

// 调用组件
type ComponentStatement struct {
	ComponentKey    string
	ComponentStruct ComponentStruct
}

// 调用组件语句
// o是上层options.
// 根据组件attr拼接出新的scope, 再执行组件
// 处理slot作用域
func (c *ComponentStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	rCtx := ctxPool.Get().(*RenderCtx)
	rCtx.Store = ctx.Store
	rCtx.Scope = o.Scope
	defer ctxPool.Put(rCtx)

	// 计算Props
	var props *Props
	if c.ComponentStruct.VBind != nil || c.ComponentStruct.Props != nil {
		props = NewProps()
		// v-bind="{id: 1}" 语法, 将计算出整个PropsR
		if c.ComponentStruct.VBind != nil {
			c.ComponentStruct.VBind.execTo(rCtx, props)
		}

		// 如果还传递了其他props, 则覆盖
		if c.ComponentStruct.Props != nil {
			c.ComponentStruct.Props.execTo(rCtx, props)
		}
	}

	// 处理slot作用域
	slots := c.ComponentStruct.Slots.WrapScope(o)

	cp, exist := ctx.Components[c.ComponentKey]
	// 没有找到组件时直接渲染自身的子组件
	if !exist {
		ctx.W.WriteString(fmt.Sprintf(`<%s data-err="not found component">`, c.ComponentKey))

		if slots != nil {
			child := slots.Default
			if child != nil {
				err := child.ExecSlot(ctx, nil)
				if err != nil {
					return nil
				}
			}
		}

		ctx.W.WriteString(fmt.Sprintf("</%s>", c.ComponentKey))
		return nil
	}

	// 执行指令
	// 指令可以修改scope/props/style/class/children
	if len(c.ComponentStruct.Directives) != 0 {
		data := &NodeData{
			Props: props,
			Slots: slots,
		}
		execDirectives(c.ComponentStruct.Directives, ctx, o.Scope, data)
		props = data.Props
		slots = data.Slots
	}

	//json.Marshal()

	// 运行组件应该重新使用新的scope
	// 和vue不同的是, props只有在子组件中申明才能在子组件中使用, 而vtpl不同, 它将所有props放置到变量域中.
	scope := ctx.NewScope()
	if props != nil {
		propsMap := props.ToMap()
		scope = scope.Extend(propsMap)
		// 使用skipMarshalMap解决循环引用时Marshal报错的问题
		scope.Set("$props", skipMarshalMap(propsMap))
	}

	return cp.Exec(ctx, &StatementOptions{
		// 此组件在声明时拥有的所有slots
		Slots: slots,
		// 此组件上的props
		// 用于:
		// - root组件使用props转为attr,
		// - slot组件收集所有的props实现作用域插槽(https://cn.vuejs.org/v2/guide/components-slots.html#%E4%BD%9C%E7%94%A8%E5%9F%9F%E6%8F%92%E6%A7%BD)
		// <slot :user="user">
		Props: props,

		// 此组件和其下的子组件所能访问到的所有变量(包括了当前组件的props)
		Scope: scope,
		// 父级作用域
		// 只有组件有父级作用域, 用来执行slot
		Parent: o,
	})
}

// 声明Slot的语句(编译时)
// <h1 v-slot:default="SlotProps"> </h1>
type SlotC struct {
	Name     string
	propsKey string
	Children Statement
}

// Slot的运行时
type Slot struct {
	*SlotC

	// 在运行时被赋值
	// Declarer 存储在当slot声明时的组件数据
	Declarer *StatementOptions
}

type ExecSlotOptions struct {
	// <slot :abc="abc"> 语法传递的slot props.
	SlotProps *Props
}

func (s *Slot) ExecSlot(ctx *StatementCtx, o *StatementOptions) error {
	if s.Children == nil {
		return nil
	}

	var no *StatementOptions
	// 将申明时的scope和传递的slot-props合并
	if s.Declarer != nil {
		no = &StatementOptions{}
		no.Slots = s.Declarer.Slots

		scope := s.Declarer.Scope
		if o != nil && o.Props != nil && s.propsKey != "" {
			scope = scope.Extend(map[string]interface{}{
				s.propsKey: o.Props.ToMap(),
			})
		}

		no.Scope = scope
	}
	return s.Children.Exec(ctx, no)
}

// 胡子语法: {{a}}
// 会进行html转义
type mustacheStatement struct {
	exp expression
}

var ctxPool sync.Pool

func init() {
	ctxPool = sync.Pool{New: func() interface{} {
		return &RenderCtx{}
	}}
}

func (i *mustacheStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	rCtx := ctxPool.Get().(*RenderCtx)
	rCtx.Store = ctx.Store
	rCtx.Scope = o.Scope
	defer ctxPool.Put(rCtx)

	//rCtx := &RenderCtx{
	//	Store: ctx.Store, Scope: o.Scope,
	//}

	r := i.exp.Exec(rCtx)

	ctx.W.WriteString(util.InterfaceToStr(r, true))
	return nil
}

// 不会转义的html语句
// 用于v-html
type rawHtmlStatement struct {
	exp expression
}

func (i *rawHtmlStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	rCtx := ctxPool.Get().(*RenderCtx)
	rCtx.Store = ctx.Store
	rCtx.Scope = o.Scope
	defer ctxPool.Put(rCtx)

	r := i.exp.Exec(rCtx)

	ctx.W.WriteString(util.InterfaceToStr(r, false))
	return nil
}

// https://cn.vuejs.org/v2/guide/components-slots.html
// SlotsC 存放传递给组件的所有Slot（默认与具名）(编译时), vue语法: <h1 v-slot:default="xxx"></h1>
type SlotsC struct {
	Default   *SlotC
	NamedSlot map[string]*SlotC
}

func (s *SlotsC) marge(x *SlotsC) {
	if x == nil {
		return
	}

	if x.Default != nil {
		s.Default = x.Default
	}

	if x.NamedSlot != nil {
		if s.NamedSlot == nil {
			s.NamedSlot = make(map[string]*SlotC, len(x.NamedSlot))
		}
		for k, xs := range x.NamedSlot {
			s.NamedSlot[k] = xs
		}
	}
}

// WrapScope 设置在slot声明时的scope, 用于在运行slot时使用声明slot时的scope
func (s *SlotsC) WrapScope(o *StatementOptions) (sr *Slots) {
	if s == nil {
		return nil
	}
	if s.Default != nil {
		sr = &Slots{
			Default: &Slot{
				SlotC:    s.Default,
				Declarer: o,
			},
		}
	}
	if s.NamedSlot != nil {
		if sr == nil {
			sr = &Slots{}
		}

		sr.NamedSlot = make(map[string]*Slot, len(s.NamedSlot))
		for k, v := range s.NamedSlot {
			sr.NamedSlot[k] = &Slot{
				SlotC:    v,
				Declarer: o,
			}
		}
	}

	return
}

type Slots struct {
	Default   *Slot // 大多数都是Default插槽，放入map里会有性能损耗，所以优化为单独一个字段
	NamedSlot map[string]*Slot
}

func (s *Slots) Get(key string) *Slot {
	if s == nil {
		return nil
	}
	if key == "default" {
		return s.Default
	}
	return s.NamedSlot[key]
}

func ParseHtmlToStatement(tpl string, options *parser.ParseVueNodeOptions) (Statement, *SlotsC, error) {
	nt, err := parser.ParseHtml(tpl)
	if err != nil {
		return nil, nil, err
	}
	vn, err := parser.ToVueNode(nt, options)
	if err != nil {
		return nil, nil, fmt.Errorf("parseToVue err: %w", err)
	}
	statement, slots, err := toStatement(vn)
	if err != nil {
		return nil, nil, err
	}
	return statement, slots, nil
}

// 执行语句(组件/Tag)所需的参数
type StatementOptions struct {
	Slots *Slots

	// 渲染组件时, 组件上的props
	// 如
	//   <Menu :data="data">
	//   <slot name="default">
	Props *Props

	// 如果是渲染tag, scope是当前组件的scope(如果在For语句中, 则也有For的scope).
	// 如果渲染其他组件, scope也是当前组件的scope.
	Scope *Scope

	// 上一层参数, 用于:
	// - 渲染slot时获取声明slot时的scope.
	// - 渲染slot时获取上一层的slots, 从中取出slot渲染. (slot组件自己的slot是备选内容)
	Parent *StatementOptions
}

// 整个渲染期间的上下文.
// Ctx贯穿整个渲染流程, 意味着每一个组件/方法/指令都可以拿到同一个ctx, 只有其中的值会变化.
type StatementCtx struct {
	Global *Scope

	Store Store

	Ctx context.Context
	W   Writer

	Components    map[string]Statement
	Directives    map[string]Directive
	CanBeAttrsKey func(k string) bool
}

func (c *StatementCtx) NewScope() *Scope {
	return NewScope(c.Global)
}

func (c *StatementCtx) Clone() *StatementCtx {
	return &StatementCtx{
		Global:        c.Global,
		Store:         c.Store,
		Ctx:           c.Ctx,
		W:             c.W,
		Components:    c.Components,
		Directives:    c.Directives,
		CanBeAttrsKey: c.CanBeAttrsKey,
	}
}

func (c *StatementCtx) Set(k string, v interface{}) {
	c.Store.Set(k, v)
}
func (c *StatementCtx) Get(k string) (v interface{}, exist bool) {
	return c.Store.Get(k)
}

type Store interface {
	Get(key string) (interface{}, bool)
	Set(key string, val interface{})
}

type MapStore map[string]interface{}

func (g MapStore) Get(key string) (interface{}, bool) {
	v, exist := g[key]
	return v, exist
}

func (g MapStore) Set(key string, val interface{}) {
	g[key] = val
}

// 如果全部是静态Props, 则能被渲染为静态字符串
func canBeStr(v *parser.VueElement) bool {
	return v.Props.IsStatic() &&
		//v.PropClass == nil &&
		//v.PropStyle == nil &&
		v.VBind == nil &&
		v.VHtml == "" &&
		v.VText == "" &&
		len(v.Directives) == 0 &&
		v.DistributionAttr == false
}

var htmlTag = map[string]struct{}{
	"html":       {},
	"head":       {},
	"header":     {},
	"footer":     {},
	"body":       {},
	"meta":       {},
	"title":      {},
	"div":        {},
	"input":      {},
	"p":          {},
	"h1":         {},
	"h2":         {},
	"h3":         {},
	"h4":         {},
	"h5":         {},
	"h6":         {},
	"hr":         {},
	"blockquote": {},
	"ul":         {},
	"li":         {},
	"span":       {},
	"script":     {},
	"link":       {},
	"a":          {},
	"object":     {},
	"button":     {},
	"img":        {},
	"i":          {},
	"center":     {},
	"table":      {},
	"tbody":      {},
	"thead":      {},
	"th":         {},
	"tr":         {},
	"td":         {},
}

// 通过Vue树，生成运行程序
// 需要做的事：
// - 简化vue树
//   - 将连在一起的静态节点预渲染为字符串
// - 预编译JS
// 原则是将运行时消耗减到最小
func toStatement(v *parser.VueElement) (Statement, *SlotsC, error) {
	slots := &SlotsC{}
	switch v.NodeType {
	case parser.RootNode:
		// Root节点只是一个虚拟节点, 不渲染自己, 直接渲染子级

		// 子集
		var sg groupStatement
		for _, c := range v.Children {
			s, slotsc, err := toStatement(c)
			if err != nil {
				return nil, nil, err
			}
			slots.marge(slotsc)
			sg.Append(s)
		}
		return sg.Finish(), slots, nil
	case parser.DoctypeNode:
		return &StrStatement{Str: v.Text}, nil, nil
	case parser.ElementNode:
		var st Statement

		// 静态节点(不是自定义组件)，则走渲染tag逻辑, 否则调用渲染组件方法
		if _, ok := htmlTag[v.Tag]; ok {
			var sg groupStatement

			// 如果没使用任何变量, 则是静态组件, 则编译成字符串
			if canBeStr(v) {
				attrs := ""

				//if len(v.Class) != 0 {
				//	attrs += v.Class.ToAttr()
				//}
				//
				//if len(v.Style) != 0 {
				//	if attrs != "" {
				//		attrs += " "
				//	}
				//	attrs += v.Style.ToAttr()
				//}

				// 静态props
				if len(v.Props) != 0 {
					attrs += genAttrFromProps(v.Props)
				}

				if attrs != "" {
					attrs = " " + attrs
				}

				sg.Append(&StrStatement{Str: fmt.Sprintf("<%s%s>", v.Tag, attrs)})

				// 子集
				for _, c := range v.Children {
					s, slotsc, err := toStatement(c)
					if err != nil {
						return nil, nil, err
					}
					slots.marge(slotsc)
					sg.Append(s)
				}

				// 单标签不需要结束
				if !parser.VoidElements[v.Tag] {
					sg.Append(&StrStatement{Str: fmt.Sprintf("</%s>", v.Tag)})
				}
			} else {
				// 动态的（依赖变量）节点渲染

				//pc, err := compileProp(v.PropClass)
				//if err != nil {
				//	return nil, nil, err
				//}
				//ps, err := compileProp(v.PropStyle)
				//if err != nil {
				//	return nil, nil, err
				//}

				// 如果 style和class动态与静态不冲突 ,并且沒有指令, 则可以将静态style/class优化为 string
				staticProp := !v.DistributionAttr && v.VBind == nil && len(v.Directives) == 0
				p, err := compileProps(v.Props, staticProp)
				if err != nil {
					return nil, nil, err
				}

				// 如果被自动分配attr, 那么直接继承上一层的props
				var vbind *vBindC
				if v.DistributionAttr {
					vbind = &vBindC{useProps: true}
				} else {
					vbind, err = compileVBind(v.VBind)
					if err != nil {
						return nil, nil, err
					}
				}

				dir, err := compileDirective(v.Directives)
				if err != nil {
					return nil, nil, err

				}

				var childStatement Statement

				if v.VHtml != "" {
					node, err := compileJS(v.VHtml)
					if err != nil {
						return nil, nil, err
					}
					childStatement = &rawHtmlStatement{
						exp: &jsExpression{node: node, code: v.VHtml},
					}
				} else if v.VText != "" {
					node, err := compileJS(v.VText)
					if err != nil {
						return nil, nil, err
					}
					childStatement = &mustacheStatement{
						exp: &jsExpression{node: node, code: v.VText},
					}
				} else {
					var childStatementG groupStatement
					for _, c := range v.Children {
						s, slotsc, err := toStatement(c)
						if err != nil {
							return nil, nil, err
						}
						slots.marge(slotsc)

						childStatementG.Append(s)
					}

					childStatement = childStatementG.Finish()
				}

				// 子集 作为default slot
				var slots *SlotsC
				if childStatement != nil {
					slots = &SlotsC{
						Default: &SlotC{
							Name:     "default",
							propsKey: "",
							Children: childStatement,
						},
						NamedSlot: nil,
					}
				}

				sg.Append(&tagStatement{
					tag: v.Tag,
					tagStruct: tagStruct{
						Props: p,
						//PropClass:   pc,
						//PropStyle:   ps,
						//StaticClass: v.Class,
						//StaticStyle: v.Style,
						Directives: dir,
						Slots:      slots,
						VBind:      vbind,
					},
				})
			}

			st = sg.Finish()
		} else {
			// 自定义组件
			var childStatement Statement

			if v.VHtml != "" {
				node, err := compileJS(v.VHtml)
				if err != nil {
					return nil, nil, err
				}
				childStatement = &rawHtmlStatement{
					exp: &jsExpression{node: node, code: v.VHtml},
				}
			} else if v.VText != "" {
				node, err := compileJS(v.VText)
				if err != nil {
					return nil, nil, err
				}
				childStatement = &mustacheStatement{
					exp: &jsExpression{node: node, code: v.VText},
				}
			} else {
				// 子集 作为default slot
				var childStatementG groupStatement
				for _, c := range v.Children {
					s, slotsc, err := toStatement(c)
					if err != nil {
						return nil, nil, err
					}
					slots.marge(slotsc)
					childStatementG.Append(s)
				}

				childStatement = childStatementG.Finish()
			}

			if v.Tag == "template" && len(v.Directives) == 0 {
				// 如果是template 并且没有自定义指令, 则可以简化语句
				st = childStatement
			} else {
				if childStatement != nil {
					slots.Default = &SlotC{
						Name:     "default",
						propsKey: "",
						Children: childStatement,
					}
				}

				vbind, err := compileVBind(v.VBind)
				if err != nil {
					return nil, nil, err
				}

				dir, err := compileDirective(v.Directives)
				if err != nil {
					return nil, nil, err
				}
				p, err := compileProps(v.Props, !v.DistributionAttr && v.VBind == nil)
				if err != nil {
					return nil, nil, err
				}

				st = &ComponentStatement{
					ComponentKey: v.Tag,
					ComponentStruct: ComponentStruct{
						Props:      p,
						VBind:      vbind,
						Directives: dir,
						Slots:      slots,
					},
				}
			}

			// 如果调用了自定义组件, 则slots就算这个自定义组件当中, 而不算在父级当中.
			slots = &SlotsC{}
		}

		if v.VIf != nil {
			ifCondition, err := compileJS(v.VIf.Condition)
			if err != nil {
				return nil, nil, err
			}
			// 解析else节点
			elseIfStatements := make([]*elseStatement, len(v.VIf.ElseIf))
			for i, f := range v.VIf.ElseIf {
				st, slotsc, err := toStatement(f.VueElement)
				if err != nil {
					return nil, nil, err
				}
				slots.marge(slotsc)

				s := &elseStatement{
					conditionCode:  f.Condition,
					condition:      nil,
					ChildStatement: st,
				}

				if f.Types == "elseif" && f.Condition != "" {
					n, err := compileJS(f.Condition)
					if err != nil {
						return nil, nil, err
					}

					s.condition = &jsExpression{node: n, code: f.Condition}
				}

				elseIfStatements[i] = s
			}

			st = &ifStatement{
				condition:      &jsExpression{node: ifCondition, code: v.VIf.Condition},
				conditionCode:  v.VIf.Condition,
				ChildStatement: st,
				ElseIf:         elseIfStatements,
			}
		}

		if v.VFor != nil {
			p, err := compileJS(v.VFor.ArrayKey)
			if err != nil {
				return nil, nil, err
			}

			st = &forStatement{
				ArrayKey:    v.VFor.ArrayKey,
				Array:       &jsExpression{node: p, code: v.VFor.ArrayKey},
				ItemKey:     v.VFor.ItemKey,
				IndexKey:    v.VFor.IndexKey,
				ChildChunks: st,
			}
		}

		if v.VSlot != nil {
			if v.VSlot.SlotName == "default" {
				slots.Default = &SlotC{
					Name:     v.VSlot.SlotName,
					propsKey: v.VSlot.PropsKey,
					Children: st,
				}
			} else {
				if slots.NamedSlot == nil {
					slots.NamedSlot = map[string]*SlotC{}
				}
				slots.NamedSlot[v.VSlot.SlotName] = &SlotC{
					Name:     v.VSlot.SlotName,
					propsKey: v.VSlot.PropsKey,
					Children: st,
				}
			}

			// 自己不是语句, 而是slot
			st = nil
		}

		return st, slots, nil
	case parser.TextNode:
		s, err := parseBeard(v.Text)
		if err != nil {
			return nil, nil, err
		}
		return s, slots, nil
	case parser.CommentNode:
		return &StrStatement{Str: v.Text}, nil, nil
	default:
		return &StrStatement{Str: fmt.Sprintf("not case NodeType: %+v", v.NodeType)}, nil, nil
	}

}

// 将胡子语法处理成多个语句
func parseBeard(txt string) (Statement, error) {
	var sg groupStatement

	if strings.Contains(txt, "{{") {
		for index, v := range strings.Split(txt, "{{") {
			if len(v) == 0 {
				continue
			}
			if index == 0 {
				sg.Append(&StrStatement{Str: v})
			} else {
				sp := strings.Split(v, "}}")
				if len(sp) == 2 {
					code := sp[0]
					if len(code) != 0 {
						node, err := compileJS(code)
						if err != nil {
							return nil, err
						}
						sg.Append(&mustacheStatement{
							exp: &jsExpression{node: node, code: code},
						})
					}
					if len(sp[1]) != 0 {
						sg.Append(&StrStatement{Str: sp[1]})
					}
				} else {
					// bad token
					sg.Append(&StrStatement{Str: v})
				}
			}
		}
	} else {
		sg.Append(&StrStatement{Str: txt})
	}

	return sg.Finish(), nil
}
