package vpl

import (
	"github.com/zbysir/vpl/internal/parser"
	"io/ioutil"
	"os"
	"testing"
)

func TestSimpleNodeVue(t *testing.T) {
	const rawPageHtml = `
<div style="color: red">
	<div class="c">
		Text
	</div>
	<div :id="id" style="color: red" :style="{'a': 1}">
		Infos:
		<ul :class="'c' + ulClass" class="b" style="color: red" >
			<li v-if="ifStart" about=a>Start <span> span </span></li>
			<li v-else about=b>Not Start</li>
			<li v-for="item in infos" :id="item.id" :key="item.id">{{item.label}}: {{item.value}}</li>
			<li v-show="a" style="top: 1px">VShow</li>
			<li>End</li>
		</ul>
	</div>
</div>
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
	//      Tag(li, [style(attr): map[top:1px]], v-show)
	//        VShow
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
