package vpl

import (
	"github.com/zbysir/vpl/internal/parser"
	"io/ioutil"
	"os"
	"testing"
)

func TestSimpleNodeVue(t *testing.T) {
	const rawPageHtml = `
<div class="c">
	Text
</div>
<div :id="id">
	Infos:
	<ul :class="'c' + ulClass">
		<li v-if="ifStart" about=a>Start <span> span </span></li>
		<li v-else about=b>Not Start</li>
		<li v-for="item in infos" :id="item.id" :key="item.id">{{item.label}}: {{item.value}}</li>
		<li>End</li>
	</ul>
</div>
`

	// 将会优化成另一个AST:
	// <div class="c">Text</div>
	// tagStart(div, {Props:[0xc00010f1a0] PropClass:<nil> PropStyle:<nil> VBind:<nil> StaticClass:[] StaticStyle:map[] Directives:[] Slots:map[]})
	// Infos:
	// tagStart(ul, {Props:[] PropClass:0xc00010f2f0 PropStyle:<nil> VBind:<nil> StaticClass:[] StaticStyle:map[] Directives:[] Slots:map[]})
	// If(ifStart)
	//  <li about="a">Start<span>span</span></li>
	// ELSE
	//  <li about="b">Not Start</li>
	// FOR(item in infos)
	//  tagStart(li, {Props:[0xc00010f560 0xc00010f5f0] PropClass:<nil> PropStyle:<nil> VBind:<nil> StaticClass:[] StaticStyle:map[] Directives:[] Slots:map[]})
	//  {{item.label}}
	//  :
	//  {{item.value}}
	//  </li>
	// <li>End</li></ul></div>

	nt, err := parser.ParseHtml(rawPageHtml)
	if err != nil {
		t.Fatal(err)
	}
	vn, err := parser.ToVueNode(nt)
	if err != nil {
		t.Fatal(err)
	}

	c, _, err := toStatement(vn)
	if err != nil {
		t.Fatal(err)
	}

	ioutil.WriteFile("statement.txt", []byte(NicePrintStatement(c, 0)), os.ModePerm)
}
