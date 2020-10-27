package parser

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestToVue(t *testing.T) {
	const rawPageHtml = `
<ul class="a">
	<li v-if="ifA" style="color: red"></li>
	<li about="a">我的名字：{{name}}</li>
	<li>我的年龄：25</li>
	<li>我的性别：男</li>
</ul>
	`
	//r:=buffer.NewReader([]byte(rawPageHtml))

	nt, err := ParseHtml(rawPageHtml)
	if err != nil {
		t.Fatal(err)
	}
	vn, err := ToVueNode(nt, &ParseVueNodeOptions{
		CanBeAttr: func(k string) bool {
			return true
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ioutil.WriteFile("parsevue_output.txt", []byte(vn.NicePrint(true, 0)), os.ModePerm)
}
