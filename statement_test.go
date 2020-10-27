package vpl

import (
	"github.com/zbysir/vpl/internal/parser"
	"io/ioutil"
	"os"
	"testing"
)

func TestSimpleNodeVue(t *testing.T) {
	const rawPageHtml = `
<body>
<div v-html="html"></div>
<template v-text="html"></template>
</body>
`

	// 将会优化成另一个AST:
	// Tag(div, [style(attr): map[color:red]], BindProps)
	//  <div class="c">Text</div>
	//  Tag(div, [id(attr): id, style(attr): map[color:red], style(attr): {'a': 1}])
	//    Infos:
	//    Tag(ul, [class(attr): 'c' + ulClass, class(attr): [b], style(attr): color: red;])
	//      If(ifStart)
	//        <li about="a">Start<span>span</span></li>
	//      Else
	//        <li about="b">Not Start</li>
	//      For(item in infos)
	//        Tag(li, [id(attr): item.id, key: item.id])
	//          {{item.label}}
	//          :
	//          {{item.value}}
	//      <li>End</li>

	nt, err := parser.ParseHtml(rawPageHtml)
	if err != nil {
		t.Fatal(err)
	}
	vn, err := parser.ToVueNode(nt, nil)
	if err != nil {
		t.Fatal(err)
	}

	c, _, err := toStatement(vn)
	if err != nil {
		t.Fatal(err)
	}

	ioutil.WriteFile("statement.txt", []byte(NicePrintStatement(c, 0)), os.ModePerm)
}
