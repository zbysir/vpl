package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zbysir/vpl"
	"io/ioutil"
	"os"
	"runtime"
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
			// 基础测试
			Name:           "main",
			IndexTpl:       `{{id}}<main v-bind="$props"></main>`,
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
<div class="c">
	Text
</div>
<div :id="id" :a='1'>
	Infos:
	<ul :class="[{'t': true},'d', 'c' + ulClass, 'b', 'a']" class="b">
		<li v-if="isStart" about=a>Starting <span> span </span></li>
		<li v-else>Not Start</li>
		<li v-if="status==='Running'">状态: Running</li>
		<li v-else-if="status==='Sleeping'">状态: Sleeping</li>
		<li v-else>状态未知: {{status}}</li>
		<template v-for="(item, index) in infos">
			{{index}}
			<li  :id="item.id" :key="item.id">{{item.label}}: {{item.value}}</li>
		</template>
		<li>End</li>
	</ul>
</div>
{{$props}}
author: {{author}}
</body>
</html>
`,
			}},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, `class="b t d cuuu a"`) {
					return errors.New("处理class有误")
				}

				return nil
			},
		},
		{
			// 测试组件
			Name: "component",
			//IndexTpl: "<main></main>",
			IndexComponent: `main`,
			Tpl: []struct {
				Name string
				Txt  string
			}{
				{
					Name: "main",
					Txt: `
<div :id="id">
	Infos:
	<Infos :infos="infos" id=123></Infos>
	<InfosX :infos="infos">我是错误的组件 {{infos.length}}</Infos>
</div>`,
				},
				{
					Name: "Infos",
					Txt: `
<!-- vue使用 v-bind="$attrs" 将属性放置到tag上, 但vtpl不能区分attrs和props, 所以只能使用props. -->
<!-- 可以通过设置Vue.Options.CanBeAttrsKey('id', 'class', 'style', 'data-*')来指定那些props转成attrs -->
<div v-bind="$props">
	<template v-for="item in infos">
		{{$index}}
		<li  :id="item.id" :key="item.id">{{item.label}}: {{item.value}}</li>
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

				return nil
			},
		},
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
	</Infos>
</div>`,
				},
				{
					Name: "Infos",
					Txt: `
<slot>默认备选</slot>
<slot name="title" :title="'Infos'"></slot>
<slot name="title2">标题2 备选</slot>
<template v-for="item in infos">
	<li :id="item.id" :key="item.id">{{item.label}}: {{item.value}}</li>
</template>
{{author}}
`,
				},
			},
			Output: "output/%s.html",
		},
		{
			// 测试 单标签
			Name:     "voidElements",
			IndexTpl: "<main></main>",
			Tpl: []struct {
				Name string
				Txt  string
			}{
				{
					Name: "main",
					Txt: `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Title</title>
</head>
<body>
</body>
</html>
`,
				},
			},
			Output: "output/%s.html",
		}, {
			// 测试 单标签
			Name:           "vHtml",
			IndexComponent: "main",
			Tpl: []struct {
				Name string
				Txt  string
			}{
				{
					Name: "main",
					Txt: `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Title</title>
</head>
<body>
<div v-html="html"></div>
<template v-text="html"></template>
</body>
</html>
`,
				},
			},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, `富文本<span>`) {
					return errors.New("VHtml指令执行有误")
				}

				if !strings.Contains(html, `富文本&lt;span`) {
					return errors.New("VText指令执行有误")
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

			props := vpl.NewProps()
			props.AppendMap(map[string]interface{}{
				"id":      "helloID",
				"ulClass": "uuu",
				"status":  "Sleeping",
				"html":    "<h1>富文本<span>-</span></h1>",
				"isStart": 1,
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

type data struct {
	C   []*data `json:"c"`
	Msg string  `json:"msg"`
}

// 100	539,790 ns/op
// 10000 53,219,775 ns/op
func BenchmarkRender(b *testing.B) {
	b.ReportAllocs()
	vue := vpl.New()
	b.Logf("compile....")
	err := vue.ComponentTxt("main", `
  <div v-bind:class="{'a': true}" class="b">
    <span class="d" v-bind:class="{c: true}" :a="1">
        {{data.msg}}
    </span>
    <div v-for="item in data.c">
      <main :data="item"></main>
    </div>
  </div>
`)

	if err != nil {
		b.Fatal(err)
	}

	var ii interface{}
	// 生成10000个数据
	index := 0
	var ds []*data
	for i := 0; i < 100; i++ {
		ds = append(ds, &data{
			C:   nil,
			Msg: fmt.Sprintf("%d", index),
		})
		index++
	}

	d := data{
		C:   ds,
		Msg: "1",
	}
	bs, _ := json.Marshal(d)
	json.Unmarshal(bs, &ii)

	props := vpl.NewProps()
	props.AppendMap(map[string]interface{}{
		"data": ii,
	})

	for i := 0; i < b.N; i++ {
		_, err := vue.RenderComponent("main", &vpl.RenderParam{
			Global: nil,
			Ctx:    context.Background(),
			Props:  props,
		})
		if err != nil {
			b.Fatal(err)
		}

		//b.Logf("%+v", html)
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	kb := 1024.0

	logstr := fmt.Sprintf("\nAlloc = %v\tTotalAlloc = %v\tSys = %v\t NumGC = %v\n\n", float64(m.Alloc)/kb, float64(m.TotalAlloc)/kb, float64(m.Sys)/kb, m.NumGC)
	b.Log(logstr)
}
