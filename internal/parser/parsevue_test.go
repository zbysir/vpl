package parser

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestToVue(t *testing.T) {
	const rawPageHtml = `
<ul>
<li v-if="ifA">  </li>
<li>我的名字：{{name}}</li>
<li>我的年龄：25</li>
<li>我的性别：男</li>
</ul>
	`
	//r:=buffer.NewReader([]byte(rawPageHtml))

	nt, err := ParseHtml(rawPageHtml)
	if err != nil {
		t.Fatal(err)
	}
	vn, err := ToVueNode(nt)
	if err != nil {
		t.Fatal(err)
	}

	ioutil.WriteFile("vue.txt", []byte(vn.NicePrint(true, 0)), os.ModePerm)

}
