# Introduction

Vpl is a template engine for golang, template syntax like [vuejs](https://vuejs.org).

- Componentization
- Powerful template syntax for the modern html
- Supports Js(Es5) expressions
- A little faster (I tried my best to optimize :)

## Installation
```
go get github.com/zbysir/vpl
```

## Getting Started
Write the `main.go` file as follows

```
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

<div style="font-size: 20px" :style="{color: color}">
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
	// Output: <!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>hello vpl</title></head><body><div style="color: red; font-size: 20px;">hello vpl</div></body></html>
}

```

Then run it.

> More examples in `/example` and `/test`

### Description of the parameters
You only need to understand a few parameters.

#### vpl.Props
```
props := vpl.NewProps()
// use Append to add a variable
props.Append("lang", "en")

// use AppendMap to add multiple variables 
props.AppendMap(map[string]interface{}{
    "title": "hello vpl",
    "color": "red",
})
```

#### vpl.RenderParam

```
vpl.RenderParam{
    Global: nil, // Defined Global Variable in this Render Content.
    Props:  props, // Props to Render Component.
}
```

## Docs
- [Syntax Reference](./doc/syntax.md)
- [Golang API](./doc/api.md)

## IntelliJ Plugin
Just use the Vuejs plugin.

## Dependencies
- github.com/robertkrimen/otto:  Used to parse Js expression.
