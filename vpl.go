package vpl

import (
	"context"
	"fmt"
	"github.com/valyala/bytebufferpool"
	"github.com/zbysir/vpl/internal/parser"
	"io/ioutil"
	"strings"
	"sync"
)

// 常驻实例, 一个程序只应该有一个实例.
// 在运行期间是无副作用的
type Vpl struct {
	components map[string]Statement

	// 类似原型链, 用于注册方法/等全局变量, 这些变量在每一个组件中都可以使用
	prototype *Scope

	// 指令
	directives map[string]Directive

	// 什么prop可以被写成attr(编译时)
	canBeAttrsKey func(k string) bool
}

type Options func(o *Vpl)

func WithCanBeAttrsKey(canBeAttr func(k string) bool) Options {
	return func(o *Vpl) {
		o.canBeAttrsKey = canBeAttr
	}
}

// New return a Vpl instance,
// This instance should be shared in multiple renderings.
// The recommended practice is to have only one Vpl instance for the whole program.
func New(options ...Options) *Vpl {
	vpl := &Vpl{
		components: map[string]Statement{
			// 模板, 直接渲染子组件
			// 注意, 所有slot执行都有"编译作用域的问题"(https://cn.vuejs.org/v2/guide/components-slots.html#%E7%BC%96%E8%AF%91%E4%BD%9C%E7%94%A8%E5%9F%9F)
			// slot是在父组件声明并会使用变量, 却在子组件中运行, 所以在执行slot时需要使用父组件环境.
			"template": FuncStatement(func(ctx *StatementCtx, o *StatementOptions) error {
				slot := o.Slots.Default
				if slot == nil {
					return nil
				}
				err := slot.ExecSlot(ctx, &ExecSlotOptions{
					SlotProps: nil,
				})
				if err != nil {
					return nil
				}
				return nil
			}),
			// <slot name="abc" :abc=123>语句
			// 注意, 所有slot执行都有"编译作用域的问题"(https://cn.vuejs.org/v2/guide/components-slots.html#%E7%BC%96%E8%AF%91%E4%BD%9C%E7%94%A8%E5%9F%9F)
			// slot是在父组件声明并会使用变量, 却在子组件中运行, 所以在执行slot时需要使用父组件环境.
			"slot": FuncStatement(func(ctx *StatementCtx, o *StatementOptions) error {
				slotName := ""
				attr, exist := o.Props.Get("name")
				if exist {
					slotName, _ = attr.(string)
				}
				var slot *Slot

				p := o.Parent
				if slotName == "" {
					slot = p.Slots.Default
				} else {
					slot = p.Slots.Get(slotName)
				}

				if slot == nil {
					// 备选内容
					fullback := o.Slots.Default
					if fullback != nil {
						err := fullback.ExecSlot(ctx, &ExecSlotOptions{
							SlotProps: nil,
						})
						if err != nil {
							return err
						}
					}

					return nil
				}

				err := slot.ExecSlot(ctx, &ExecSlotOptions{
					SlotProps: o.Props,
				})
				if err != nil {
					return nil
				}
				return nil
			}),
			// <parallel> 并行语句
			// 被parallel组件包裹起来的子组件都会被同时渲染,
			// 假如有3个耗时组件分别用时 3/2/1 s, 如果都使用parallel组件包裹起来, 最终渲染耗时应该是 3 s.
			"parallel": FuncStatement(func(ctx *StatementCtx, o *StatementOptions) error {
				s := NewChanSpan()
				go func() {
					ctx := ctx.Clone()
					ctx.W = NewListWriter()
					child := o.Slots.Default
					if child == nil {
						s.Done("")
						return
					}
					err := child.ExecSlot(ctx, &ExecSlotOptions{})
					if err != nil {
						s.Done(fmt.Sprintf("err: %+v", err))
					} else {
						s.Done(ctx.W.Result())
					}
				}()

				ctx.W.WriteSpan(s)

				return nil
			}),
		},
		prototype:  NewScope(nil),
		directives: map[string]Directive{},
	}

	for _, o := range options {
		o(vpl)
	}

	if vpl.canBeAttrsKey == nil {
		vpl.canBeAttrsKey = DefaultCanBeAttr
	}
	return vpl
}

var DefaultCanBeAttr = func(k string) bool {
	if k == "id" {
		return true
	}
	if strings.HasPrefix(k, "data-") {
		return true
	}

	return false
}

func (v *Vpl) Component(name string, c Statement) (err error) {
	v.components[name] = c
	return nil
}

// Declare a component by file
func (v *Vpl) ComponentFile(name string, path string) (err error) {
	fileBs, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("readFile err: %w", err)
	}

	return v.ComponentTxt(name, string(fileBs))
}

// Declare a component by txt
func (v *Vpl) ComponentTxt(name string, txt string) (err error) {
	s, err := ParseHtmlToStatement(txt, &parser.ParseVueNodeOptions{
		CanBeAttr: v.canBeAttrsKey,
	})
	if err != nil {
		return
	}

	return v.Component(name, s)
}

// Global 设置全局变量, 在所有的组件中都生效
// 也可用于设置全局方法
func (v *Vpl) Global(name string, val interface{}) () {
	v.prototype.Set(name, val)
	return
}

// Function 是对 Global 方法设置全局方法的再次封装
func (v *Vpl) Function(name string, val Function) () {
	v.prototype.Set(name, val)
	return
}

// Directive 声明一个指令
func (v *Vpl) Directive(name string, val Directive) () {
	v.directives[name] = val
	return
}

func (v *Vpl) NewScope() *Scope {
	s := NewScope(v.prototype)
	return s
}

// tpl e.g.: <main v-bind="$props"></main>
func (v *Vpl) RenderTpl(tpl string, p *RenderParam) (html string, err error) {
	statement, err := ParseHtmlToStatement(tpl, &parser.ParseVueNodeOptions{
		CanBeAttr: v.canBeAttrsKey,
	})
	if err != nil {
		return "", fmt.Errorf("parseHtmlToStatement err: %w", err)
	}

	var w = NewListWriter()

	global := v.NewScope()

	if p.Global != nil {
		global = global.Extend(p.Global)
	}
	ctx := &StatementCtx{
		Global:        global,
		Store:         nil,
		Ctx:           p.Ctx,
		W:             w,
		Components:    v.components,
		Directives:    v.directives,
		CanBeAttrsKey: v.canBeAttrsKey,
	}

	propsMap := p.Props.ToMap()
	// 将所有props放入scope
	scope := ctx.NewScope().Extend(propsMap)
	// copyMap是为了让$props和scope的value不相等, 否则在打印$props就会出现循环引用.
	scope.Set("$props", skipMarshalMap(propsMap))

	err = statement.Exec(ctx, &StatementOptions{
		Slots:  nil,
		Props:  p.Props,
		Scope:  scope,
		Parent: nil,
	})

	if err != nil {
		err = fmt.Errorf("RenderTpl err: %w", err)
		return
	}
	html = w.Result()
	return
}

// 渲染一个已经编译好的组件
func (v *Vpl) RenderComponent(component string, p *RenderParam) (html string, err error) {
	statement := ComponentStatement{
		ComponentKey: component,
		ComponentStruct: ComponentStruct{
			Props:     nil,
			// 将所有Props传递到组件中
			VBind:      &vBindC{useProps: true},
			Directives: nil,
			Slots:      nil,
		},
	}

	var w = NewListWriter()

	global := v.NewScope()

	if p.Global != nil {
		global = global.Extend(p.Global)
	}
	ctx := &StatementCtx{
		Global:        global,
		Store:         p.Store,
		Ctx:           p.Ctx,
		W:             w,
		Components:    v.components,
		Directives:    v.directives,
		CanBeAttrsKey: v.canBeAttrsKey,
	}

	scope := v.NewScope()
	scope.Set("$props", p.Props)
	err = statement.Exec(ctx, &StatementOptions{
		Slots:     nil,
		Props:     p.Props,
		Scope:     scope,
		Parent:    nil,
	})

	if err != nil {
		err = fmt.Errorf("RenderComponent err: %w", err)
		return
	}

	html = w.Result()
	return
}

// 支持同时写Span和string的Write
// 优化多个字符串拼接
type ListWriter struct {
	s bytebufferpool.ByteBuffer

	// 可以优化为链表, 减少append消耗
	spans []Span
}

func (p *ListWriter) Result() string {
	if len(p.spans) == 0 {
		s := p.s.String()
		p.s.Reset()
		return s
	}

	if p.s.Len() != 0 {
		p.spans = append(p.spans, &StringSpan{s: p.s.String()})
		p.s.Reset()
	}

	var s bytebufferpool.ByteBuffer
	for _, p := range p.spans {
		s.WriteString(p.Result())
	}

	return s.String()
}

func (p *ListWriter) WriteString(s string) {
	p.s.WriteString(s)
}

func (p *ListWriter) WriteSpan(span Span) {
	if p.s.Len() != 0 {
		p.spans = append(p.spans, &StringSpan{s: p.s.String()})
		p.s.Reset()
	}
	p.spans = append(p.spans, span)
}

type StringSpan struct {
	s string
}

func (s *StringSpan) Result() string {
	return s.s
}

func NewListWriter() *ListWriter {
	return &ListWriter{}
}

type ChanSpan struct {
	c       chan string
	getOnce sync.Once
	setOnce sync.Once
	r       string
}

func (p *ChanSpan) Result() string {
	p.getOnce.Do(func() {
		p.r = <-p.c
	})
	return p.r
}

func (p *ChanSpan) Done(s string) {
	p.setOnce.Do(func() {
		p.c <- s
	})
}

func NewChanSpan() *ChanSpan {
	return &ChanSpan{
		c:       make(chan string, 1),
		getOnce: sync.Once{},
		setOnce: sync.Once{},
	}
}

type RenderParam struct {
	// 声明本次渲染的全局变量, 和vpl.Global()功能类似, 在所有组件中都有效.
	// 可以用来存放诸如版本号/作者等全部组件都可能需要访问的数据, 还可以存放方法.
	// Global设置的值会覆盖vpl.Global()设置的值.
	//
	// Q: 如何区分应该使用RenderParam.Global设置全局变量还是vpl.Global()设置全局变量?
	// A:
	//  根据这个"全局"变量的真正范围而定.
	//  如果这个变量是"整个程序"的全局变量, 如一个全局方法, 那么它应该使用vpl.Global()设置
	//  如果这个变量是"这一次渲染过程中"的全局变量, 如在渲染每个页面时的页面ID, 那么它应该使用RenderParam.Global设置.
	Global map[string]interface{}
	// 用于在整个运行环境共享变量, 如在一个方法/指令中读取另一个方法/指令里存储的数据
	Store Store

	Ctx   context.Context
	Props *Props
}
