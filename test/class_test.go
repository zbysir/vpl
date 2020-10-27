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

func TestClassStyle(t *testing.T) {
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
			// 基础测试
			Name:           "class",
			IndexComponent: `main`,
			Tpl: []struct {
				Name string
				Txt  string
			}{{
				Name: "main",
				Txt: `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Title</title>
</head>
<body>
<div class="a" style="top: 1px" :class="'b'">
	Text
</div>
<div style="top: 10px" :style="{color: color}">
	Text
</div>

</body>
</html>
`,
			}},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, `style="color: red; top: 10px;"`) {
					return errors.New("处理style有误")
				}

				if !strings.Contains(html, `class="a b"`) {
					return errors.New("处理class有误")
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
