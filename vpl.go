package vpl

import (
	"context"
	"fmt"
	"github.com/valyala/bytebufferpool"
	"github.com/zbysir/vpl/internal/compiler"
	"github.com/zbysir/vpl/internal/parser"
	"github.com/zbysir/vpl/internal/util"
	"io/ioutil"
	"sync"
)

// 常驻实例, 一个程序只应该有一个实例.
// 在运行期间是无副作用的
type Vpl struct {
	components map[string]compiler.Statement

	// 类似原型链, 用于注册方法/等全局变量, 这些变量在每一个组件中都可以使用
	prototype *compiler.Scope

	// 指令
	directives map[string]parser.Directive

	// 什么prop可以被写成attr
	canBeAttr func(key string, data interface{}) bool
}

func New() *Vpl {
	return &Vpl{
		components: map[string]compiler.Statement{
			// 模板, 直接渲染子组件
			// 注意, 所有slot执行都有"编译作用域的问题"(https://cn.vuejs.org/v2/guide/components-slots.html#%E7%BC%96%E8%AF%91%E4%BD%9C%E7%94%A8%E5%9F%9F)
			// slot是在父组件声明并会使用变量, 却在子组件中运行, 所以在执行slot时需要使用父组件环境.
			"template": compiler.FuncStatement(func(ctx *compiler.StatementCtx, o *compiler.StatementOptions) error {
				slot := o.Slots.Default()
				if slot == nil {
					return nil
				}
				err := slot.ExecSlot(ctx, &compiler.ExecSlotOptions{
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
			"slot": compiler.FuncStatement(func(ctx *compiler.StatementCtx, o *compiler.StatementOptions) error {
				slotName := ""
				attr, exist := o.Props.Get("name")
				if exist {
					slotName, _ = attr.Val.(string)
				}
				if slotName == "" {
					slotName = "default"
				}

				p := o.Parent
				slot := p.Slots.Get(slotName)
				if slot == nil {
					// 备选内容
					fullback := o.Slots.Default()
					if fullback != nil {
						err := fullback.ExecSlot(ctx, &compiler.ExecSlotOptions{
							SlotProps: nil,
						})
						if err != nil {
							return err
						}
					}

					return nil
				}

				err := slot.ExecSlot(ctx, &compiler.ExecSlotOptions{
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
			"parallel": compiler.FuncStatement(func(ctx *compiler.StatementCtx, o *compiler.StatementOptions) error {
				s := NewChanSpan()
				go func() {
					ctx := ctx.Clone()
					ctx.W = NewListWriter()
					err := o.Slots.Default().ExecSlot(ctx, &compiler.ExecSlotOptions{})
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
		prototype:  compiler.NewScope(),
		directives: nil,
	}
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

func (v *Vpl) Component(name string, c compiler.Statement) (err error) {
	v.components[name] = c
	return nil
}

func (v *Vpl) ComponentFile(name string, path string) (err error) {
	fileBs, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("readFile err: %w", err)
	}

	return v.ComponentTxt(name, string(fileBs))
}

func (v *Vpl) ComponentTxt(name string, txt string) (err error) {
	s, err := compiler.ParseHtmlToStatement(txt)
	if err != nil {
		return
	}

	return v.Component(name, s)
}

// 设置全局变量
// 在所有的组件中都生效
func (v *Vpl) Global(name string, val interface{}) () {
	v.prototype.Set(name, val)
	return
}
func (v *Vpl) NewScope() *compiler.Scope {
	s := compiler.NewScope()
	s.Parent = v.prototype
	return s
}

type RenderParam struct {
	// 本次渲染的全局变量, 在所有组件中都有效
	Global map[string]interface{}

	Ctx   context.Context
	Props *compiler.Props
}

// tpl e.g.: <main v-bind="$props"></main>
func (v *Vpl) RenderTpl(tpl string, p *RenderParam) (html string, err error) {
	statement, err := compiler.ParseHtmlToStatement(tpl)
	if err != nil {
		return "", fmt.Errorf("parseHtmlToStatement err: %w", err)
	}

	var w = NewListWriter()

	global := compiler.NewScope()
	global.Parent = v.prototype

	if p.Global != nil {
		global = global.Extend(p.Global)
	}
	ctx := &compiler.StatementCtx{
		Global:     global,
		Ctx:        p.Ctx,
		W:          w,
		Components: v.components,
	}

	propsMap := p.Props.ToMap()
	// 将所有props放入scope
	scope := ctx.NewScope().Extend(propsMap)
	// copyMap是为了让$props和scope的value不相等, 否则在打印$props就会出现循环引用.
	scope.Set("$props", util.CopyMap(propsMap))

	err = statement.Exec(ctx, &compiler.StatementOptions{
		Slots:     nil,
		Props:     p.Props,
		PropClass: nil,
		PropStyle: nil,
		Scope:     scope,
		Parent:    nil,
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
	statement := compiler.ComponentStatement{
		ComponentKey: component,
		ComponentStruct: compiler.ComponentStruct{
			Props:     nil,
			PropClass: nil,
			PropStyle: nil,
			// 将所有Props传递到组件中
			VBind:      &compiler.VBindC{Val: compiler.NewRawExpression(p.Props)},
			Directives: nil,
			Slots:      nil,
		},
	}

	var w = NewListWriter()

	global := v.NewScope()

	if p.Global != nil {
		global = global.Extend(p.Global)
	}
	ctx := &compiler.StatementCtx{
		Global:     global,
		Ctx:        p.Ctx,
		W:          w,
		Components: v.components,
	}
	err = statement.Exec(ctx, &compiler.StatementOptions{
		Slots:     nil,
		Props:     nil,
		PropClass: nil,
		PropStyle: nil,
		Scope:     nil,
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
	spans []compiler.Span
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
		s.Write(util.UnsafeStrToBytes(p.Result()))
	}

	return s.String()
}

func (p *ListWriter) WriteString(s string) {
	p.s.Write(util.UnsafeStrToBytes(s))
}

func (p *ListWriter) WriteSpan(span compiler.Span) {
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

func NewProps() *compiler.Props {
	return compiler.NewProps()
}
