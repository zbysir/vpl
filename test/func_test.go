package test

import (
	"context"
	"errors"
	"fmt"
	"github.com/zbysir/vpl"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

type Cases struct {
	Name           string
	IndexTpl       string
	IndexComponent string
	Tpl            []struct {
		Name string
		Txt  string
	}
	Output  string
	Checker func(html string) error
}

// 测试function调用
func TestFunc(t *testing.T) {
	cases := []Cases{
		{
			// 测试方法调用
			Name:           "function",
			IndexTpl:       "",
			IndexComponent: `main`,
			Tpl: []struct {
				Name string
				Txt  string
			}{{
				Name: "main",
				Txt:  `appendName: {{appendName('z', 'bysir')}} | fullName: {{fullName}} {{setVar('fullName', fullName)}} | getVar: {{getVar('fullName')}}`,
			}},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, "appendName: z|bysir") {
					return errors.New("方法返回有误")
				}
				if !strings.Contains(html, "fullName: z|bysir") {
					return errors.New("方法设置Scope有误")
				}
				if !strings.Contains(html, "getVar: z|bysir") {
					return errors.New("方法设置Store有误")
				}

				return nil
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			vue := vpl.New()
			t.Logf("compile....")

			for _, tp := range c.Tpl {
				err := vue.ComponentTxt(tp.Name, tp.Txt)
				if err != nil {
					t.Fatal(err)
				}
			}

			vue.Global("author", "bysir")
			vue.Function("appendName", func(ctx *vpl.RenderCtx, args ...interface{}) interface{} {
				fullName := fmt.Sprintf("%s|%s", args[0], args[1])
				ctx.Scope.Set("fullName", fullName)
				return fullName
			})
			vue.Function("setVar", func(ctx *vpl.RenderCtx, args ...interface{}) interface{} {
				ctx.Store.Set(args[0].(string), args[1])
				return ""
			})
			vue.Function("getVar", func(ctx *vpl.RenderCtx, args ...interface{}) interface{} {
				x, _ := ctx.Store.Get(args[0].(string))
				return x
			})

			t.Logf("run....")

			props := vpl.NewProps()
			var html string
			var err error
			if c.IndexComponent != "" {
				html, err = vue.RenderComponent(c.IndexComponent, &vpl.RenderParam{
					Global: nil,
					Ctx:    context.Background(),
					Props:  props,
					Store:  vpl.MapStore{},
				})
			} else {
				html, err = vue.RenderTpl(c.IndexTpl, &vpl.RenderParam{
					Global: nil,
					Ctx:    context.Background(),
					Props:  props,
					Store:  vpl.MapStore{},
				})
			}

			if err != nil {
				t.Fatal(err)
			}

			if c.Checker != nil {
				err = c.Checker(html)
				if err != nil {
					t.Fatal(err)
				}
			}
			ioutil.WriteFile(fmt.Sprintf(c.Output, c.Name), []byte(html), os.ModePerm)

			t.Logf("%s", html)
		})

	}
}
