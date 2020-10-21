package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zbysir/vpl"
	"github.com/zbysir/vpl/internal/compiler"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestDirective(t *testing.T) {
	cases := []struct {
		Name           string
		IndexComponent string
		Tpl            []struct {
			Name string
			Txt  string
		}
		Output  string
		Checker func(html string) error
	}{
		{
			Name:           "directive",
			IndexComponent: `main`,
			Tpl: []struct {
				Name string
				Txt  string
			}{{
				Name: "main",
				Txt: `
<body>
<div v-animate="{iteration: 20, duration: 20}">
	Text
</div>
<div style="top: 10px" v-style-important="{color: color}">
	Text
</div>
<script v-js-set:$color="color">
	Text
</script>


</body>
`,
			}},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, `data-wow-iteration`) {
					return errors.New("自定义指令执行v-animate有误")
				}

				if !strings.Contains(html, `color: red !important`) {
					return errors.New("自定义指令v-style-important执行有误")
				}

				return nil
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			vue := vpl.New()
			t.Logf("compile....")

			type Animate struct {
				Type      string  `json:"type"`
				Direction string  `json:"direction"`
				Iteration int     `json:"iteration"`
				Delay     float64 `json:"delay"`
				Duration  float64 `json:"duration"`
			}
			vue.Directive("v-animate", func(ctx *compiler.DirectivesCtx, binding *compiler.DirectivesBinding) {
				var a Animate
				Copy(binding.Value, &a)

				ctx.Props.Append("data-wow-iteration", fmt.Sprintf("%v", a.Iteration))
				ctx.Props.Append("data-wow-delay", fmt.Sprintf("%0.2fs", a.Delay))
				ctx.Props.Append("data-wow-duration", fmt.Sprintf("%0.2fs", a.Duration))
			})

			vue.Directive("v-style-important", func(ctx *compiler.DirectivesCtx, binding *compiler.DirectivesBinding) {
				m := binding.Value.(map[string]interface{})

				for k, v := range m {
					ctx.Style.Add(k, v.(string)+" !important")
				}
			})

			vue.Directive("v-js-set", func(ctx *compiler.DirectivesCtx, binding *compiler.DirectivesBinding) {
				bs, _ := json.Marshal(binding.Value)

				(*ctx.Slots)["default"] = &compiler.VSlotStatementR{
					VSlot: &compiler.VSlotStruct{
						Name:     "",
						Children: &compiler.StrStatement{Str: fmt.Sprintf("var %s=%s;", binding.Arg, bs)},
					},
					ScopeWhenDeclaration: &compiler.Scope{},
				}
			})

			for _, tp := range c.Tpl {
				err := vue.ComponentTxt(tp.Name, tp.Txt)
				if err != nil {
					t.Fatal(err)
				}
			}

			t.Logf("run....")

			props := vpl.NewProps()
			props.AppendMap(map[string]interface{}{
				"css":   []interface{}{"b", "c"},
				"color": "red",
			})
			var html string
			var err error

			html, err = vue.RenderComponent(c.IndexComponent, &vpl.RenderParam{
				Global: nil,
				Ctx:    context.Background(),
				Props:  props,
			})

			if err != nil {
				t.Fatal(err)
			}
			ioutil.WriteFile(fmt.Sprintf(c.Output, c.Name), []byte(html), os.ModePerm)

			t.Logf("%s", html)
		})

	}
}

func Copy(src, dst interface{}) (err error) {
	//json := jsoniter.ConfigCompatibleWithStandardLibrary
	bs, err := json.Marshal(src)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, dst)
	if err != nil {
		return
	}

	return
}
