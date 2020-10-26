package vpl

import (
	"github.com/zbysir/vpl/internal/parser"
	"io/ioutil"
	"os"
	"testing"
)

func TestSimpleNodeVue(t *testing.T) {
	const rawPageHtml = `
<div>
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
</div>
`

	// 将会优化成另一个AST:
	// <div class="c">Text</div>
	// Tag(div, [id: id])
	//   Infos:
	//   Tag(ul, [])
	//     If(ifStart)
	//       <li about="a">Start<span>span</span></li>
	//     Else
	//       <li about="b">Not Start</li>
	//     For(item in infos)
	//       Tag(li, [id: item.id, key: item.id])
	//         {{item.label}}
	//         :
	//         {{item.value}}
	//     <li>End</li>

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
