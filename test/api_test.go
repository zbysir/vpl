package test_test

import (
	"github.com/zbysir/vpl"
	"testing"
)

// 测试RenderComponent Api
func TestRenderComponent(t *testing.T) {
	layout := `<!DOCTYPE html>
<html :lang="lang">
<head>
    <meta charset="UTF-8">
    <title>{{title}}</title>
</head>
<body>

<slot name="title"></slot>

{{global}}

<slot></slot>
</body>`

	// 如果直接将组件作为slot渲染, 那么它的作用域就丢失了.
	// 所以lang为空
	contentTpl := `
<h1 v-slot:title>
Title from content {{global}} {{lang}}
</h1>
`

	v := vpl.New()
	err := v.ComponentTxt("layout", layout)
	if err != nil {
		return
	}

	props := vpl.NewProps()
	props.Append("title", "title")
	props.Append("lang", "zh")

	contentStatement, slots, err := vpl.ParseHtmlToStatement(contentTpl, nil)
	if err != nil {
		t.Fatal(err)
	}

	html, err := v.RenderComponent("layout", &vpl.RenderParam{
		Global: map[string]interface{}{
			"global": "global",
		},
		Store: nil,
		Ctx:   nil,
		Props: props,
		Slots: &vpl.SlotsC{
			Default: &vpl.SlotC{
				Children: contentStatement,
			},
			NamedSlot: slots.NamedSlot,
		},
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	if html != `<!DOCTYPE html><html lang="zh"><head><meta charset="UTF-8"><title>title</title></head><body><h1>Title from content global null</h1>global</body></html>` {
		t.Fatalf("组件渲染有误 输出: %s", html)
	}
	t.Logf("%s", html)
}
