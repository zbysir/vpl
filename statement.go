package vpl

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/robertkrimen/otto/ast"
	"github.com/zbysir/vpl/internal/js"
	"github.com/zbysir/vpl/internal/lib/log"
	"github.com/zbysir/vpl/internal/parser"
	"github.com/zbysir/vpl/internal/util"
	"strings"
)

// 执行每一个块的上下文
type Scope struct {
	Parent *Scope
	Value  map[string]interface{}
}

func (s *Scope) Get(k string) interface{} {
	return s.GetDeep(k)
}

func NewScope() *Scope {
	return &Scope{
		Parent: nil,
		Value:  map[string]interface{}{},
	}
}

// 获取作用域中的变量
// 会向上查找
func (s *Scope) GetDeep(k ...string) (v interface{}) {
	var rootExist bool
	var ok bool

	curr := s
	for curr != nil {
		v, rootExist, ok = js.ShouldLookInterface(curr.Value, k...)
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
	s.Value[k] = v
}

type RenderCtx struct {
	W     Writer
	Scope *Scope // 向当前scope声明一个值
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
	Key     string
	ValCode string
	Val     expression
}

type propsC []*propC

// 执行编译之后的PropsC, 返回数值PropsR.
func (r propsC) exec(scope *Scope) *Props {
	if len(r) == 0 {
		return nil
	}
	pr := NewProps()
	for _, p := range r {
		pr.Append(p.Key, p.exec(scope).Val)
	}
	return pr
}

func (r *propC) exec(scope *Scope) *Prop {
	if r == nil {
		return &Prop{}
	}
	return &Prop{Key: r.Key, Val: r.Val.Exec(scope)}
}

// 数值Prop
// 执行PropC会得到PropR
type Prop struct {
	Key string
	Val interface{}
}

type Props struct {
	orderKey []string         // 在生成attr时会用到顺序
	data     map[string]*Prop // 存储map有利于快速取值
}

func NewProps() *Props {
	return &Props{
		// 减少扩展slice的cpu消耗
		orderKey: make([]string, 0, 0),
		data:     map[string]*Prop{},
	}
}

func (r *Props) ForEach(cb func(index int, r *Prop)) {
	for index, k := range r.orderKey {
		cb(index, r.data[k])
	}
	return
}

func (r *Props) ToMap() map[string]interface{} {
	if r == nil {
		return nil
	}
	m := make(map[string]interface{})
	for k, p := range r.data {
		m[k] = p.Val
	}
	return m
}

func (r *Props) Append(k string, v interface{}) {
	o, exist := r.data[k]
	if exist {
		o.Val = v
	} else {
		r.data[k] = &Prop{
			Key: k,
			Val: v,
		}

		r.orderKey = append(r.orderKey, k)
	}
}

// 无序添加多个props
func (r *Props) AppendMap(mp map[string]interface{}) {
	keys := util.GetSortedKey(mp)

	for _, k := range keys {
		v := mp[k]
		r.Append(k, v)
	}
}

// 有序添加多个props
func (r *Props) appendProps(ps *Props) {
	if ps == nil {
		return
	}
	ps.ForEach(func(index int, p *Prop) {
		r.Append(p.Key, p.Val)
	})
}

func (r *Props) Get(key string) (*Prop, bool) {
	v, exist := r.data[key]
	return v, exist
}

func compileProps(p parser.Props) (propsC, error) {
	pc := make(propsC, len(p))
	for i, v := range p {
		p, err := compileProp(v)
		if err != nil {
			return nil, err
		}
		pc[i] = p
	}
	return pc, nil
}

func compileProp(p *parser.Prop) (*propC, error) {
	if p == nil {
		return nil, nil
	}
	var valExpression expression

	if p.IsStatic {
		valExpression = &rawExpression{raw: p.Val}
	} else {
		if p.Val != "" {
			node, err := js.CompileJS(p.Val)
			if err != nil {
				return nil, fmt.Errorf("parseJs err: %w", err)
			}
			valExpression = &jsExpression{node: node, code: p.Val}
		} else {
			valExpression = &nullExpression{}
		}
	}
	return &propC{
		Key:     p.Key,
		ValCode: p.Val,
		Val:     valExpression,
	}, nil
}

func compileVBind(v *parser.VBind) (*vBindC, error) {
	if v == nil {
		return nil, nil
	}

	if v.Val == "" {
		return nil, nil
	}

	node, err := js.CompileJS(v.Val)
	if err != nil {
		return nil, fmt.Errorf("parseJs err: %w", err)
	}

	return &vBindC{Val: &jsExpression{node: node, code: v.Val}}, nil
}

func compileDirective(ds parser.Directives) (directivesC, error) {
	if len(ds) == 0 {
		return nil, nil
	}

	pc := make(directivesC, len(ds))
	for i, v := range ds {
		node, err := js.CompileJS(v.Value)
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
	// Props: 无论动态还是静态, 都是Props (除了style和class, style和class为了优化性能, 需要特殊处理)
	// 静态的attr也处理成Props是为了保持顺序, 当然也是为了减少概念
	//
	//  如: <div id="abc" :data-id="id" :style="{left: '1px'}">
	//  其中 Props 值为: id, data-id
	//  其中 PropStyle 值为: style
	//
	// 另外tag上的 Props 都会被转为html attr
	Props     propsC
	PropClass *propC
	PropStyle *propC
	VBind     *vBindC

	// 静态class, 将会和动态class合并
	StaticClass parser.Class
	StaticStyle parser.Styles

	Directives directivesC
	Slots      SlotsC
}

type directiveC struct {
	Name  string     // v-animate
	Value expression // {'a': 1}
	Arg   string     // v-set:arg
}

type directivesC []directiveC

// 组件的属性
type ComponentStruct struct {
	// Props: 无论动态还是静态, 都是Props (除了style和class, style和class为了优化性能, 需要特殊处理)
	//
	//  如: <Menu :data="data" id="abc" :style="{left: '1px'}">
	//  其中 Props 值为: data, id
	//  其中 PropStyle 值为: style
	Props propsC
	// PropClass指动态class
	//  如 <Menu :class="['a','b']" class="c">
	//  那么PropClass的值是: ['a', 'b']
	PropClass *propC
	PropStyle *propC
	VBind     *vBindC
	// 静态class, 将会和动态class合并
	StaticClass parser.Class
	StaticStyle parser.Styles

	Directives directivesC
	// 传递给这个组件的Slots
	Slots SlotsC
}

// VBind 语法, 一次传递多个prop
// v-bind='{id: id, 'other-attr': otherAttr}'
// 有两个特殊用法:
//  v-bind='$props': 将父组件的 props(不包括class和style) 一起传给子组件
type vBindC struct {
	Val expression
}

func (v *vBindC) Exec(s *Scope) *Props {
	if v == nil {
		return nil
	}
	pr := NewProps()
	b := v.Val.Exec(s)
	switch t := b.(type) {
	case map[string]interface{}:
		pr.AppendMap(t)
	case *Props:
		return t
	}

	return pr
}

type expression interface {
	Exec(ctx *Scope) interface{}
}

// 原始值
type rawExpression struct {
	raw interface{}
}

func (r *rawExpression) Exec(*Scope) interface{} {
	return r.raw
}

func newRawExpression(raw interface{}) *rawExpression {
	return &rawExpression{raw: raw}
}

type jsExpression struct {
	node ast.Node
	code string
}

func (r *jsExpression) Exec(scope *Scope) interface{} {
	v, err := js.RunJsExpression(r.node, scope)
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

func (r *nullExpression) Exec(*Scope) interface{} {
	return nil
}

// vue语法会被编译成一组Statement
// 为了避免多次运行造成副作用, 所有的Statement在运行时都不应该被修改任何值
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

func (s *StrStatement) AppendStr(str string) {
	s.Str += str
}

// tag开始块
type tagStatement struct {
	tag       string
	tagStruct tagStruct
}

func execDirectives(ds directivesC, ctx *StatementCtx, scope *Scope, o *NodeData) {
	for _, v := range ds {
		val := v.Value.Exec(scope)
		d, exist := ctx.Directives[v.Name]
		if exist {
			d(
				&RenderCtx{
					Scope: scope,
					W:     ctx.W,
				},
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
	Class *Class //
	Style *Styles
	Slots *Slots
}

func (t *tagStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	// 将tagStruct根据scope变量渲染出属性
	var attrs strings.Builder

	// 处理class
	cla := parser.NewClass()
	if len(t.tagStruct.StaticClass) != 0 {
		cla.Merge(t.tagStruct.StaticClass)
	}
	if t.tagStruct.PropClass != nil {
		claProp := getClassFromProps(t.tagStruct.PropClass.exec(o.Scope).Val)
		cla.Merge(claProp)
	}

	// 处理style
	sty := parser.Styles{}
	if len(t.tagStruct.StaticStyle) != 0 {
		sty.Merge(t.tagStruct.StaticStyle)
	}
	if t.tagStruct.PropStyle != nil {
		styProp := getStyleFromProps(t.tagStruct.PropStyle.exec(o.Scope).Val)
		if len(styProp) != 0 {
			sty.Merge(styProp)
		}
	}
	// 处理attr
	// 计算Props
	props := NewProps()
	// v-bind="{id: 1}" 语法, 将计算出整个PropsR
	if t.tagStruct.VBind != nil {
		props.appendProps(t.tagStruct.VBind.Exec(o.Scope))
	}

	if len(t.tagStruct.Props) != 0 {
		props.appendProps(t.tagStruct.Props.exec(o.Scope))
	}

	slotsR := t.tagStruct.Slots.WrapScope(o.Scope)

	// 执行指令
	// 指令可以修改scope/props/style/class/children
	if len(t.tagStruct.Directives) != 0 {
		execDirectives(t.tagStruct.Directives, ctx, o.Scope, &NodeData{
			Props: props,
			Class: &cla,
			Style: &sty,
			Slots: &slotsR,
		})
	}

	// 生成 attrs
	if len(cla) != 0 {
		attrs.WriteString(cla.ToAttr())
	}

	if len(sty) != 0 {
		if attrs.Len() != 0 {
			attrs.Write([]byte(" "))
		}
		attrs.WriteString(sty.ToAttr())
	}

	props.ForEach(func(index int, p *Prop) {
		if attrs.Len() != 0 {
			attrs.Write([]byte(" "))
		}
		attrs.WriteString(p.Key)

		if p.Val != nil {
			attrs.WriteString(`="`)

			switch v := p.Val.(type) {
			case string:
				attrs.WriteString(v)
			default:
				attrs.WriteString(util.InterfaceToStr(v, true))
			}
			attrs.WriteString(`"`)
		}
	})

	tagStart := "<" + t.tag

	if attrs.Len() != 0 {
		tagStart += " " + attrs.String()
	}

	tagStart += ">"

	ctx.W.WriteString(tagStart)

	// 子节点
	children := slotsR.Get("default")
	if children != nil {
		err := children.ExecSlot(ctx, nil)
		if err != nil {
			return err
		}
	}

	ctx.W.WriteString("</" + t.tag + ">")
	return nil
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
	r := i.condition.Exec(o.Scope)
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
			if util.InterfaceToBool(ef.condition.Exec(o.Scope)) {
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
	arr := f.Array.Exec(o.Scope)

	for index, item := range util.Interface2Slice(arr) {
		scope := o.Scope.Extend(map[string]interface{}{
			f.IndexKey: index,
			f.ItemKey:  item,
		})

		err := f.ChildChunks.Exec(ctx, &StatementOptions{
			Scope: scope,
		})
		if err != nil {
			return err
		}
	}

	return nil
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
		return nil
	}

	if len(g.s) == 1 {
		return g.s[0]
	}
	return g
}

func (g *groupStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	for _, v := range g.s {
		err := v.Exec(ctx, o)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *groupStatement) Last() Statement {
	if len(g.s) == 0 {
		return nil
	}

	return g.s[len(g.s)-1]
}

func (g *groupStatement) First() Statement {
	if len(g.s) == 0 {
		return nil
	}

	return g.s[0]
}

// Append 拼接一个新的语句到组里, 如果有连续的字符串语句 则会合并成为一个字符串语句.
func (g *groupStatement) Append(st Statement) {
	if st == nil {
		return
	}
	switch appT := st.(type) {
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
	// 计算Props
	propsR := NewProps()
	// v-bind="{id: 1}" 语法, 将计算出整个PropsR
	if c.ComponentStruct.VBind != nil {
		propsR.appendProps(c.ComponentStruct.VBind.Exec(o.Scope))
	}

	// 如果还传递了其他props, 则覆盖
	if c.ComponentStruct.Props != nil {
		propsR.appendProps(c.ComponentStruct.Props.exec(o.Scope))
	}

	propClass := c.ComponentStruct.PropClass.exec(o.Scope)
	propStyle := c.ComponentStruct.PropClass.exec(o.Scope)

	// 处理slot作用域
	slots := c.ComponentStruct.Slots.WrapScope(o.Scope)

	cp, exist := ctx.Components[c.ComponentKey]
	// 没有找到组件时直接渲染自身的子组件
	if !exist {
		ctx.W.WriteString(fmt.Sprintf(`<%s data-err="not found component">`, c.ComponentKey))

		child := slots.Default()
		if child != nil {
			err := child.ExecSlot(ctx, &ExecSlotOptions{
				SlotProps: nil,
			})
			if err != nil {
				return nil
			}
		}

		ctx.W.WriteString(fmt.Sprintf("</%s>", c.ComponentKey))
		return nil
	}

	// 执行指令
	// 指令可以修改scope/props/style/class/children
	if len(c.ComponentStruct.Directives) != 0 {
		execDirectives(c.ComponentStruct.Directives, ctx, o.Scope, &NodeData{
			Props: propsR,
			Class: &o.StaticClass,
			Style: &o.StaticStyle,
			Slots: &slots,
		})
	}

	// 运行组件应该重新使用新的scope
	// 和vue不同的是, props只有在子组件中申明才能在子组件中使用, 而vtpl不同, 它将所有props放置到变量域中.
	props := propsR.ToMap()
	scope := ctx.NewScope().Extend(props)
	// copyMap是为了让$props和scope的value不相等, 否则在打印$props就会出现循环引用.
	scope.Set("$props", util.CopyMap(props))

	return cp.Exec(ctx, &StatementOptions{
		// 此组件在声明时拥有的所有slots
		Slots: slots,
		// 此组件上的props
		// 用于:
		// - root组件使用props转为attr,
		// - slot组件收集所有的props实现作用域插槽(https://cn.vuejs.org/v2/guide/components-slots.html#%E4%BD%9C%E7%94%A8%E5%9F%9F%E6%8F%92%E6%A7%BD)
		// <slot :user="user">
		Props:     propsR,
		PropClass: propClass,
		PropStyle: propStyle,

		// 静态class, 在渲染到tag上时会与动态class合并
		StaticClass: o.StaticClass,
		StaticStyle: o.StaticStyle,
		// 此组件和其下的子组件所能访问到的所有变量(包括了当前组件的props)
		Scope: scope,
		// 父级作用域
		// 只有组件有父级作用域, 用来执行slot
		Parent: o,
	})
}

// 声明Slot的语句(编译时)
// v-slot:default="SlotProps"
type vSlotC struct {
	Name     string
	propsKey string
	Children Statement
}

// Slot的运行时
type Slot struct {
	Name     string
	propsKey string
	Children Statement

	// 在运行时被赋值
	ScopeWhenDeclaration *Scope
}

type ExecSlotOptions struct {
	// 如果在渲染Slot组件, 则需要传递slot props.
	SlotProps *Props
}

func (s *Slot) ExecSlot(ctx *StatementCtx, o *ExecSlotOptions) error {
	if s.ScopeWhenDeclaration == nil {
		panic(fmt.Sprintf("VSlotStatment should call Slot.SetScope to set scope befor Exec"))
	}
	scope := s.ScopeWhenDeclaration
	if o != nil && o.SlotProps != nil {
		scope = scope.Extend(map[string]interface{}{
			s.propsKey: o.SlotProps.ToMap(),
		})
	}

	return s.Children.Exec(ctx, &StatementOptions{
		Scope: scope,
	})
}

// 胡子语法: {{a}}
// 会进行html转义
type mustacheStatement struct {
	exp expression
}

func (i *mustacheStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	r := i.exp.Exec(o.Scope)

	ctx.W.WriteString(util.InterfaceToStr(r, true))
	return nil
}

// 不会转义的html语句
// 用于v-html
type rawHtmlStatement struct {
	exp expression
}

func (i *rawHtmlStatement) Exec(ctx *StatementCtx, o *StatementOptions) error {
	r := i.exp.Exec(o.Scope)

	ctx.W.WriteString(util.InterfaceToStr(r, false))
	return nil
}

// https://cn.vuejs.org/v2/guide/components-slots.html
// Slots 存放传递给组件的所有Slot, vue语法: <h1 v-slot:default="xxx"></h1>
type SlotsC map[string]*vSlotC

// WrapScope 设置在slot声明时的scope
func (s SlotsC) WrapScope(o *Scope) (sr Slots) {
	if len(s) == 0 {
		return nil
	}
	sr = Slots{}
	for k, v := range s {
		sr[k] = &Slot{
			Name:                 v.Name,
			propsKey:             v.propsKey,
			Children:             v.Children,
			ScopeWhenDeclaration: o,
		}
	}
	return
}

type Slots map[string]*Slot

func (s Slots) Default() *Slot {
	return s.Get("default")
}

func (s Slots) Get(key string) *Slot {
	if s == nil {
		return nil
	}
	return s[key]
}

func ParseHtmlToStatement(tpl string) (Statement, error) {
	nt, err := parser.ParseHtml(tpl)
	if err != nil {
		return nil, err
	}
	vn, err := parser.ToVueNode(nt)
	if err != nil {
		return nil, fmt.Errorf("parseToVue err: %w", err)
	}
	statement, _, err := toStatement(vn)
	if err != nil {
		return nil, err
	}
	return statement, nil
}

// 执行语句(组件/Tag)所需的参数
type StatementOptions struct {
	Slots Slots

	// 渲染组件时, 组件上的props
	// 如<Menu :data="data">
	//   <slot name="default">
	// 非组件时不使用
	Props     *Props
	PropClass *Prop
	PropStyle *Prop

	StaticClass Class
	StaticStyle Styles

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

	Components map[string]Statement
	Directives map[string]Directive
}

func (c *StatementCtx) NewScope() *Scope {
	s := NewScope()
	s.Parent = c.Global
	return s
}

func (c *StatementCtx) Clone() *StatementCtx {
	return &StatementCtx{
		Global:     c.Global,
		Store:      c.Store,
		Ctx:        c.Ctx,
		W:          c.W,
		Components: c.Components,
		Directives: c.Directives,
	}
}

func (c *StatementCtx) Set(k string, v interface{}) {
	c.Store.Set(k, v)
}
func (c *StatementCtx) Get(k string) (v interface{}, exist bool) {
	return c.Store.Get(k)
}

type Store map[string]interface{}

func (g Store) Get(key string) (interface{}, bool) {
	v, exist := g[key]
	return v, exist
}

func (g Store) Set(key string, val interface{}) {
	g[key] = val
}

// 如果全部是静态Props, 则能被渲染为静态字符串
func canBeStr(v *parser.VueElement) bool {
	return v.Props.IsStatic() &&
		v.PropClass == nil &&
		v.PropStyle == nil &&
		v.VBind == nil &&
		v.VHtml == "" &&
		v.VText == "" &&
		len(v.Directives) == 0
}

var htmlTag = map[string]struct{}{
	"html":   {},
	"head":   {},
	"footer": {},
	"body":   {},
	"meta":   {},
	"title":  {},
	"div":    {},
	"input":  {},
	"p":      {},
	"h1":     {},
	"ul":     {},
	"li":     {},
	"span":   {},
	"script": {},
	"link":   {},
}

// 通过Vue树，生成运行程序
// 需要做的事：
// - 简化vue树
//   - 将连在一起的静态节点预渲染为字符串
// - 预编译JS
// 原则是将运行时消耗减到最小
func toStatement(v *parser.VueElement) (Statement, SlotsC, error) {
	slots := SlotsC{}
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
			for k, v := range slotsc {
				slots[k] = v
			}
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

				if len(v.Class) != 0 {
					attrs += v.Class.ToAttr()
				}

				if len(v.Style) != 0 {
					if attrs != "" {
						attrs += " "
					}
					attrs += v.Style.ToAttr()
				}

				// 静态props
				if len(v.Props) != 0 {
					if attrs != "" {
						attrs += " "
					}
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
					for k, v := range slotsc {
						slots[k] = v
					}
					sg.Append(s)
				}

				// 单标签不需要结束
				if !parser.VoidElements[v.Tag] {
					sg.Append(&StrStatement{Str: fmt.Sprintf("</%s>", v.Tag)})
				}
			} else {
				// 动态的（依赖变量）节点渲染

				pc, err := compileProp(v.PropClass)
				if err != nil {
					return nil, nil, err
				}
				ps, err := compileProp(v.PropStyle)
				if err != nil {
					return nil, nil, err
				}

				p, err := compileProps(v.Props)
				if err != nil {
					return nil, nil, err
				}

				vbind, err := compileVBind(v.VBind)
				if err != nil {
					return nil, nil, err
				}

				dir, err := compileDirective(v.Directives)
				if err != nil {
					return nil, nil, err

				}

				var childStatement Statement

				if v.VHtml != "" {
					node, err := js.CompileJS(v.VHtml)
					if err != nil {
						return nil, nil, err
					}
					childStatement = &rawHtmlStatement{
						exp: &jsExpression{node: node, code: v.VHtml},
					}
				} else if v.VText != "" {
					node, err := js.CompileJS(v.VText)
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
						for k, v := range slotsc {
							slots[k] = v
						}
						childStatementG.Append(s)
					}

					childStatement = childStatementG.Finish()
				}

				// 子集 作为default slot
				slot := map[string]*vSlotC{
					"default": {
						Name:     "default",
						propsKey: "",
						Children: childStatement,
					},
				}

				sg.Append(&tagStatement{
					tag: v.Tag,
					tagStruct: tagStruct{
						Props:       p,
						PropClass:   pc,
						PropStyle:   ps,
						StaticClass: v.Class,
						StaticStyle: v.Style,
						Directives:  dir,
						Slots:       slot,
						VBind:       vbind,
					},
				})
			}

			st = sg.Finish()
		} else {
			// 自定义组件
			pc, err := compileProp(v.PropClass)
			if err != nil {
				return nil, nil, err
			}
			ps, err := compileProp(v.PropStyle)
			if err != nil {
				return nil, nil, err
			}

			p, err := compileProps(v.Props)
			if err != nil {
				return nil, nil, err
			}

			var childStatement Statement

			if v.VHtml != "" {
				node, err := js.CompileJS(v.VHtml)
				if err != nil {
					return nil, nil, err
				}
				childStatement = &rawHtmlStatement{
					exp: &jsExpression{node: node, code: v.VHtml},
				}
			} else if v.VText != "" {
				node, err := js.CompileJS(v.VText)
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
					for k, v := range slotsc {
						slots[k] = v
					}
					childStatementG.Append(s)
				}

				childStatement = childStatementG.Finish()
			}

			if childStatement != nil {
				slots["default"] = &vSlotC{
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

			st = &ComponentStatement{
				ComponentKey: v.Tag,
				ComponentStruct: ComponentStruct{
					Props:       p,
					PropClass:   pc,
					PropStyle:   ps,
					VBind:       vbind,
					StaticClass: v.Class,
					StaticStyle: v.Style,
					Directives:  dir,
					Slots:       slots,
				},
			}

			// 如果调用了自定义组件, 则slots就算这个自定义组件当中, 而不算在父级当中.
			slots = SlotsC{}
		}

		if v.VIf != nil {
			ifCondition, err := js.CompileJS(v.VIf.Condition)
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
				for k, v := range slotsc {
					slots[k] = v
				}

				s := &elseStatement{
					conditionCode:  f.Condition,
					condition:      nil,
					ChildStatement: st,
				}

				if f.Types == "elseif" && f.Condition != "" {
					n, err := js.CompileJS(f.Condition)
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
			p, err := js.CompileJS(v.VFor.ArrayKey)
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
			slots[v.VSlot.SlotName] = &vSlotC{
				Name:     v.VSlot.SlotName,
				propsKey: v.VSlot.PropsKey,
				Children: st,
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
						node, err := js.CompileJS(code)
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
