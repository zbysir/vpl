package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zbysir/vpl"
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
	v-animate
</div>
<div style="top: 10px" v-style-important="{color: color}">
	v-style-important
</div>
<script v-js-set:$color="color">
	v-js-set
</script>

<div v-show="show" style="top: 10px">
	v-show
</div>

<div v-let:testData="'hello'">
	v-let:{{testData}}
</div>


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

				if !strings.Contains(html, `display: none; top: 10px;`) {
					return errors.New("自定义指令v-show执行有误")
				}

				if !strings.Contains(html, `v-let:hello`) {
					return errors.New("自定义指令v-let执行有误")
				}

				return nil
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			v := vpl.New()
			t.Logf("compile....")

			type Animate struct {
				Type      string  `json:"type"`
				Direction string  `json:"direction"`
				Iteration int     `json:"iteration"`
				Delay     float64 `json:"delay"`
				Duration  float64 `json:"duration"`
			}
			v.Directive("animate", func(ctx *vpl.RenderCtx, nodeData *vpl.NodeData, binding *vpl.DirectivesBinding) {
				var a Animate
				Copy(binding.Value, &a)

				nodeData.Props.Append("data-wow-iteration", fmt.Sprintf("%v", a.Iteration))
				nodeData.Props.Append("data-wow-delay", fmt.Sprintf("%0.2fs", a.Delay))
				nodeData.Props.Append("data-wow-duration", fmt.Sprintf("%0.2fs", a.Duration))
			})

			v.Directive("style-important", func(ctx *vpl.RenderCtx, nodeData *vpl.NodeData, binding *vpl.DirectivesBinding) {
				m := binding.Value.(map[string]interface{})

				for k, v := range m {
					nodeData.Style.Add(k, v.(string)+" !important")
				}
			})

			v.Directive("show", func(ctx *vpl.RenderCtx, nodeData *vpl.NodeData, binding *vpl.DirectivesBinding) {
				if binding.Value == false {
					nodeData.Style.Add("display", "none")
				}
			})
			v.Directive("let", func(ctx *vpl.RenderCtx, nodeData *vpl.NodeData, binding *vpl.DirectivesBinding) {
				ctx.Scope.Set(binding.Arg, binding.Value)
			})

			v.Directive("js-set", func(ctx *vpl.RenderCtx, nodeData *vpl.NodeData, binding *vpl.DirectivesBinding) {
				bs, _ := json.Marshal(binding.Value)

				nodeData.Slots.Default = &vpl.Slot{
					Name:                 "",
					Children:             &vpl.StrStatement{Str: fmt.Sprintf("var %s=%s;", binding.Arg, bs)},
					ScopeWhenDeclaration: &vpl.Scope{},
				}
			})

			for _, tp := range c.Tpl {
				err := v.ComponentTxt(tp.Name, tp.Txt)
				if err != nil {
					t.Fatal(err)
				}
			}

			t.Logf("run....")

			props := vpl.NewProps()
			props.AppendMap(map[string]interface{}{
				"css":   []interface{}{"b", "c"},
				"color": "red",
				"show":  false,
			})
			var html string
			var err error

			html, err = v.RenderComponent(c.IndexComponent, &vpl.RenderParam{
				Global: nil,
				Ctx:    context.Background(),
				Props:  props,
			})

			if err != nil {
				t.Fatal(err)
			}
			ioutil.WriteFile(fmt.Sprintf(c.Output, c.Name), []byte(html), os.ModePerm)

			t.Logf("%s", html)

			if c.Checker != nil {
				err = c.Checker(html)
				if err != nil {
					t.Fatal(err)
				}
			}
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
