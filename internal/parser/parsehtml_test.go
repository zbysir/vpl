package parser

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	const rawPageHtml = `<!doctype html>
	<html>
	<head>
		<meta charset="utf-8">
		123123
		<title>Pagser Title</title>
		<meta name="keywords" content="golang,pagser,goquery,html,page,parser,colly">
	</head>
	
	<body>
	<!-- comment -->
		<h1>H1 Pagser Example <span>span</span></h1>
	<Br/>
	<img />
	<input>123123</input>
	<input />
	<div> bad close </span>
		<div class="navlink">
			<div class="container">
				<ul class="clearfix">
					<li :id='' class="tt" data-b dataC><a href="/">Index</a></li>
					<li @id='2'><a href="/list/web" title="web site">Web page</a></li>
					<li id='3'><a href="/list/pc" title="pc page">Pc Page</a></li>
					<li id='4'><a href="/list/mobile" title="mobile page">Mobile Page</a></li>
				</ul>
			</div>
		</div>
	
		<select>
		  <slot></slot>
		</select>
	</body>
	</html>
	`

	nt, err := ParseHtml(rawPageHtml)
	if err != nil {
		t.Fatal(err)
	}

	ioutil.WriteFile("parsehtml_output.txt", []byte(nt.NicePrint(0)), os.ModePerm)

	t.Logf("%s", nt.NicePrint(0))
}
