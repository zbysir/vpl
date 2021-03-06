package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zbysir/vpl"
	"github.com/zbysir/vpl/internal/lib/log"
	"io/ioutil"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBase(t *testing.T) {
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
<!-- $props由于存在循环引用的问题，不支持打印 -->
{{$props}}author: {{author}}
</body>
</html>
`,
			}},
			Output: "output/%s.html",
			Checker: func(html string) error {
				if !strings.Contains(html, `class="t d cuuu b a b`) {
					return errors.New("处理class有误")
				}

				return nil
			},
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
		},
		{
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
<body>
<div v-html="html"></div>
<template v-text="html"></template>
</body>
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

func TestRender(t *testing.T) {
	vue := vpl.New()
	t.Logf("compile....")
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
		t.Fatal(err)
	}

	var ii interface{}
	// 生成10000个数据
	index := 0
	var ds []*data
	for i := 0; i < 1000; i++ {
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

	time.Sleep(1 * time.Second)

	props := vpl.NewProps()
	props.AppendMap(map[string]interface{}{
		"data":  ii,
		"a":     1,
		"class": "ccc",
	})

	log.Infof("run")

	html, err := vue.RenderComponent("main", &vpl.RenderParam{
		Global: nil,
		Ctx:    context.Background(),
		Props:  props,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%+v", html)

	// 启动一个 http server
	// http://localhost:6060/debug/pprof/
	//if err := http.ListenAndServe(":6060", nil); err != nil {
	//	log.Fatal(err)
	//}
}

// 100	539,790 ns/op
// 10000 53,219,775 ns/op
// -- 2020-10-23
// 100 565072 ns/op
// -- 2020-10-24 删除掉多余的NewProps()
// 511415 ns/op	  482172 B/op	    6983 allocs/op(WIN)
// -- 2020-10-24 删除掉copyMap
// 453217 ns/op	  402995 B/op	    6177 allocs/op(WIN)
// -- 2020-10-24 使用pool管理RenderCtx
// 474725 ns/op	  390092 B/op	    5774 allocs/op(WIN)
// -- 2020-10-24 优化slot存储方式
// 386870 ns/op	  339223 B/op	    5268 allocs/op(MAC)
// 400494 ns/op	  327072 B/op	    5266 allocs/op(Win)
// -- 2020-10-16 支持分发props为attr
// 495476 ns/op	  379483 B/op	    6279 allocs/op(WIN)
// -- 2020-10-27 优化在tag上的props执行
// 323031 ns/op	  227109 B/op	    3448 allocs/op
// -- 2020-10-28 优化在tag上slot执行
// 378267 ns/op	  227844 B/op	    3247 allocs/op
func BenchmarkRender(b *testing.B) {
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

	b.ResetTimer()
	b.ReportAllocs()

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
