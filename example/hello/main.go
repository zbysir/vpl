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
	
	<ul>
		<li v-for="(item, index) in list">
			{{item}} - {{index}}
		</li>
	</ul>
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
		"id":    "content",
		"lang":  "en",
		"list":  []interface{}{1, 2, 3},
		// All data used by Vpl must be a golang base types.
		// Use vpl.Copy convert a complex structure to a structure containing only basic types.
		"list2": vpl.Copy([3]int{1, 2, 3}),
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
	// Output: <!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>hello vpl</title></head><body><div id="content" style="color: red; font-size: 20px;"><span>color is red</span><ul><li>1 - 0</li><li>2 - 1</li><li>3 - 2</li></ul></div></body></html>
}
