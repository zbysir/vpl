package test

import (
	"context"
	"errors"
	"fmt"
	"github.com/zbysir/vpl"
	"github.com/zbysir/vpl/internal/compiler"
	"github.com/zbysir/vpl/internal/js"
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
				Txt:  `appendName: {{appendName('z', 'bysir')}}`,
			}},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, "z|bysir") {
					return errors.New(fmt.Sprintf("want zbysir, but: %+v", html))
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

			t.Logf("run....")

			props := compiler.NewProps()
			props.AppendMap(map[string]interface{}{
				"appendName": js.Function(func(args ...interface{}) interface{} {
					return fmt.Sprintf("%s|%s", args[0], args[1])
				}),
			})
			var html string
			var err error
			if c.IndexComponent != "" {
				html, err = vue.RenderComponent(c.IndexComponent, &vpl.RenderParam{
					Global: nil,
					Ctx:    context.Background(),
					Props:  props,
				})
			} else {
				html, err = vue.RenderTpl(c.IndexTpl, &vpl.RenderParam{
					Global: nil,
					Ctx:    context.Background(),
					Props:  props,
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
