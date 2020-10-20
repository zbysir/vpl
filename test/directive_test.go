package test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zbysir/vpl"
	"github.com/zbysir/vpl/internal/compiler"
	"github.com/zbysir/vpl/internal/lib/log"
	"io/ioutil"
	"os"
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
<div v-animate="{iteration: 20}">
	Text
</div>
<div style="top: 10px" :style="{color: color}">
	Text
</div>

</body>
`,
			}},
			Output: "output/%s.html",
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
			vue.Directive("v-animate", func(ctx *vpl.StatementCtx, options *vpl.StatementOptions, binding *compiler.DirectivesBinding) {
				var a Animate
				Copy(binding.Value, &a)

				log.Infof("a %+v", a)
				options.Props.Append("data-wow-iteration", fmt.Sprintf("%v", a.Iteration))
				options.Props.Append("data-wow-delay", fmt.Sprintf("%0.2fs", a.Delay))
				options.Props.Append("data-wow-duration", fmt.Sprintf("%0.2fs", a.Duration))
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
