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

func TestComponent(t *testing.T) {
	cases := []struct {
		Name           string
		IndexTpl       string
		IndexComponent string
		Tpl            []struct {
			Name string
			Txt  string
		}
		Output  string
		Checker func(html string) error
	}{
		{
			// 测试组件
			Name:           "component",
			IndexComponent: `main`,
			Tpl: []struct {
				Name string
				Txt  string
			}{
				{
					Name: "main",
					Txt: `
<div>
	Infos:
	<Infos :infos="infos" :id="id" :class=[cla] :style="{color: 'red'}"></Infos>
	<InfosX :infos="infos">我是错误的组件 {{infos.length}}</Infos>
</div>`,
				},
				{
					Name: "Infos",
					Txt: `
<div class="a" style="top: 1px">
	<template v-for="item in infos">
		{{$index}}
		<li :id="item.id" :key="item.id">{{item.label}}: {{item.value}}</li>
	</template>
</div>
`,
				},
			},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, `性别: 男`) {
					return errors.New("调用组件有误")
				}

				if !strings.Contains(html, `是错误的组件 2`) {
					return errors.New("处理Slot有误")
				}

				if !strings.Contains(html, `class="a propsClass"`) {
					return errors.New("处理class继承有误")
				}

				if !strings.Contains(html, `style="color: red; top: 1px;"`) {
					return errors.New("处理style继承有误")
				}

				if !strings.Contains(html, `id="id"`) {
					return errors.New("处理attr继承有误")
				}

				return nil
			},
		},
		{
			// 测试组件-fragment, 多个root节点
			Name:           "component_fragment",
			IndexComponent: `main`,
			Tpl: []struct {
				Name string
				Txt  string
			}{
				{
					Name: "main",
					Txt: `
<div>
	<Infos :infos="infos" :id=id :class=[cla] :style="{color: 'red'}"></Infos>
</div>`,
				},
				{
					Name: "Infos",
					Txt: `
<div class="a" v-bind="$props" id="internal" style="top: 2px"></div>
<div class="b" style="top: 1px">
	<template v-for="item in infos">
		{{$index}}
		<li :id="item.id" :key="item.id">{{item.label}}: {{item.value}}</li>
	</template>
</div>
`,
				},
			},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, `性别: 男`) {
					return errors.New("调用组件有误")
				}

				if !strings.Contains(html, `class="a propsClass"`) {
					return errors.New("处理class继承有误")
				}

				if !strings.Contains(html, `style="color: red; top: 2px;"`) {
					return errors.New("处理style继承有误")
				}

				if !strings.Contains(html, `id="id"`) {
					return errors.New("处理attr继承有误")
				}

				return nil
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			vue := vpl.New(vpl.WithCanBeAttrsKey(func(k string) bool {
				if k == "id" {
					return true
				}
				if strings.HasPrefix(k, "data") {
					return true
				}

				return false
			}))
			t.Logf("compile....")

			for _, tp := range c.Tpl {
				err := vue.ComponentTxt(tp.Name, tp.Txt)
				if err != nil {
					t.Fatal(err)
				}
			}

			vue.Global("author", "bysir")

			t.Logf("run....")

			props := vpl.NewProps()
			props.AppendMap(map[string]interface{}{
				"id":    "id",
				"cla":   "propsClass",
				"cla1":  "compClass",
				"class": []interface{}{"abc"},
				"infos": []interface{}{
					map[string]interface{}{
						"id":    "sex",
						"label": "性别",
						"value": "男",
					},
					map[string]interface{}{
						"id":    "age",
						"label": "年龄",
						"value": "25",
					},
				},
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

			ioutil.WriteFile(fmt.Sprintf(c.Output, c.Name), []byte(html), os.ModePerm)

			t.Logf("%s", html)

			if c.Checker != nil {
				err := c.Checker(html)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestComponentDynamic(t *testing.T) {
	cases := []struct {
		Name           string
		IndexTpl       string
		IndexComponent string
		Tpl            []struct {
			Name string
			Txt  string
		}
		Output  string
		Checker func(html string) error
	}{
		{
			// 动态组件
			Name:           "component_dynamic",
			IndexComponent: `main`,
			Tpl: []struct {
				Name string
				Txt  string
			}{
				{
					Name: "main",
					Txt: `
<div>
	<component
      is="Infos" :id=id :class=[cla] :style="{color: 'red'}">
		<template #default=prop> main {{cla}} {{prop.a}} </template>
	</component>
</div>`,
				},
				{
					Name: "Infos",
					Txt: `
<div class="infos" style="top: 2px">
	<item>
		<h1 #default="props" :id=props.id>
			{{props['data-a']}}
			<slot v-bind=props>fullback</slot>
		</h1>
	</item>
</div>
`,
				},
				{
					Name: "item",
					Txt: `
<div class="item">
	<slot :id=1 a=1 c=3 b=2 data-c=4 data-a=2>
	</slot>
</div>
`,
				},
			},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, `class="infos propsClass"`) {
					return errors.New("处理class继承有误")
				}

				if !strings.Contains(html, `style="color: red; top: 2px;"`) {
					return errors.New("处理style继承有误")
				}

				if !strings.Contains(html, `<div class="item"><h1 id="1">2`) {
					return errors.New("处理slot有误")
				}

				if !strings.Contains(html, `<div class="item"><h1 id="1">2main propsClass 1</h1></div>`) {
					return errors.New("处理嵌套slot有误")
				}

				return nil
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			vue := vpl.New(vpl.WithCanBeAttrsKey(func(k string) bool {
				if k == "id" {
					return true
				}
				if strings.HasPrefix(k, "data") {
					return true
				}

				return false
			}))
			t.Logf("compile....")

			for _, tp := range c.Tpl {
				err := vue.ComponentTxt(tp.Name, tp.Txt)
				if err != nil {
					t.Fatal(err)
				}
			}

			vue.Global("author", "bysir")

			t.Logf("run....")

			props := vpl.NewProps()
			props.AppendMap(map[string]interface{}{
				"id":    "id",
				"cla":   "propsClass",
				"cla1":  "compClass",
				"class": []interface{}{"abc"},
				"infos": []interface{}{
					map[string]interface{}{
						"id":    "sex",
						"label": "性别",
						"value": "男",
					},
					map[string]interface{}{
						"id":    "age",
						"label": "年龄",
						"value": "25",
					},
				},
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

			ioutil.WriteFile(fmt.Sprintf(c.Output, c.Name), []byte(html), os.ModePerm)

			t.Logf("%s", html)

			if c.Checker != nil {
				err := c.Checker(html)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
