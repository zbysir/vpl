# Go API

## Vpl Instance
```
import "github.com/zbysir/vpl"
v := vpl.New()
```

> The recommended practice is to have only one Vpl instance for the whole program.

## Declare a component
```
vue := vpl.New()
// by file
vue.ComponentFile("app", "./app.vue")
// by txt
vue.ComponentTxt("app", `
<div> hello {{name}} <div>
`)
```

## Render a component
```
props := vpl.NewProps()
props.Append("name", "tom")
html, err := v.RenderComponent("app", &vpl.RenderParam{
    Props:  props,
})
```

## vpl.Props
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

## vpl.RenderParam

```
vpl.RenderParam{
    Global: nil, // Defined Global Variable in this Render Context.
    Props:  props, // Props to Render Component.
}
```
