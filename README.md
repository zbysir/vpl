# Vpl
[![Go Report Card](https://goreportcard.com/badge/github.com/zbysir/vpl)](https://goreportcard.com/report/github.com/zbysir/vpl)

Vpl is a [Vuejs](https://vuejs.org)-syntax like template-engine for Golang.

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

```go
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
	// Output: <!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>hello vpl</title></head><body><div style="color: red; font-size: 20px;"><span>color is red</span></div></body></html>
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

## With Go features
Let's add some go features to vpl.

### Parallel
The advantage of go is concurrency, can vpl use it?

YES! Use the `<parallel>` component.

Let's see this example:
```vue
<div>
    <div>
        <!-- Some things took 1s -->
        {{ sleep(1) }} 
    </div>
    <div>
        <!-- Some things took 2s -->
        {{ sleep(2) }} 
    </div>
</div>
```
If the template is executed in order, it will take 3s. To parallel them, you can wrap them with `parallel` component.

```vue
<div>
    <parallel>
        <div>
            <!-- Some things took 1s -->
            {{ sleep(1) }} 
        </div>
    </parallel>
    <parallel>
        <div>
            <!-- Some things took 2s -->
            {{ sleep(2) }} 
        </div>
    </parallel>
</div>
```
Now it only takes 2s.

## Docs
- [Syntax Reference](./doc/syntax.md)
- [Golang API](./doc/api.md)
- [Vpl Internals](./doc/internal.md)

## IntelliJ Plugin
Just use the Vuejs plugin.

## Dependencies
- github.com/robertkrimen/otto:  Used to parse Js expression.
