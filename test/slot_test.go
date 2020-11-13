package test

import (
	"context"
	"fmt"
	"github.com/zbysir/vpl"
	"io/ioutil"
	"os"
	"testing"
)

func TestSlot(t *testing.T) {
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
			// 测试 slot
			// - 具名插槽
			// - 备选
			// - 插槽作用域
			Name: "slot",
			// https://cn.vuejs.org/v2/api/#v-bind
			// 通过 $props 将父组件的 props 一起传给子组件
			// 或者通过 $scope 将父组件中所有变量一起传递给子组件
			IndexTpl: "<main v-bind='$props'></main>",
			//IndexComponent: "main",
			Tpl: []struct {
				Name string
				Txt  string
			}{
				{
					Name: "main",
					Txt: `
<div :id="id">
	<Infos :infos="infos">
		<!-- SlotStatement -->
		<h1 v-slot:title="props">({{infos.length}})条信息 {{props.title}}:</h1>
author2: {{author}}
	</Infos>
author1: {{author}}
</div>`,
				},
				{
					Name: "Infos",
					Txt: `
author3: {{author}}
<slot>默认备选</slot>
<slot name="title" :title="'Infos'"></slot>
<slot name="title2">标题2 备选</slot>
<template v-for="item in infos">
	<li :id="item.id" :key="item.id">{{item.label}}: {{item.value}}</li>
</template>
`,
				},
			},
			Output: "output/%s.html",
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

			props := vpl.NewProps()
			props.AppendMap(map[string]interface{}{
				"id": "helloID",
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
