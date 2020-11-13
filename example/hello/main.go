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
	<span v-if="color=='red'">
        color is red
    </span>
	<span v-else>
        color is {{color}}
    </span>
</div>

</body>
</html>
`)
	if err != nil {
		panic(err)
	}

	props := vpl.NewProps()
	props.AppendMap(map[string]interface{}{
		"title": "hello vpl",
		"color": "red",
		"id": "content",
		"lang": "en",
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
	// Output: <!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>hello vpl</title></head><body><div id="content" style="color: red; font-size: 20px;"><span>color is red</span></div></body></html>
}