package main

import (
	"context"
	"github.com/zbysir/vpl"
)

func main() {
	v := vpl.New()

	err := v.ComponentTxt("app", `
<!DOCTYPE html>
<html :lang="lang">
<head>
  <meta charset="UTF-8">
  <title>{{title}}</title>
</head>
<body>

<div :id="id" style="font-size: 20px" :style="{color: color}">
	hello vpl
</div>

</body>
</html>
`)
	if err != nil {
		panic(err)
	}

	props := vpl.NewProps()
	props.Append("lang", "en")
	props.AppendMap(map[string]interface{}{
		"title": "hello vpl",
		"color": "red",
		"id":    "content",
	})

	html, err := v.RenderComponent("app", &vpl.RenderParam{
		Global: nil,
		Ctx:    context.Background(),
		Props:  props,
	})
	if err != nil {
		panic(err)
	}

	print(html)
	// Output: <!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>hello vpl</title></head><body><div id="content" style="color: red; font-size: 20px;">hello vpl</div></body></html>
}
