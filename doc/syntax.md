> Almost all template syntax comes from [vuejs](https://vuejs.org/), please refer to it. Here is just a preview.

## Control Structures

#### if / else if / else
```vue
<div v-if="fooVariable">
    fooVariable is truthy
</div>
<div v-else-if="barVariable">
    barVariable is truthy
</div>
<div v-else>
    both fooVariable or barVariable are flasey
</div>
```

> tip: If you don't want to generate div tag, use `template` instead of `div`.

#### range
```vue
<ul>
  <li v-for="item in list">{{item}}</li>
</ul>
```
```vue
<ul>
  <li v-for="(item, index) in list">{{index}}: {{item}}</li>
</ul>
```

## Component
defined component:
```go
v := vpl.New()

err := v.ComponentTxt("myComponent", `
<h1> Hello </h1>
`)
```

use the component:
```vue
<div>
  <myComponent style="color: red"></myComponent>
</div>
```

### Dynamic-Component
call component by a variable name.
```vue
<component :is="'myComponent'"></component>
```

## Slot
Component A:
```vue
<div>
  <componentB>
    Tom
  </componentB>
</div>
```

Component B:
```vue
<div>
  Hello: 
  <slot></slot>
</div>
```

When the componentB renders, <slot></slot> will be replaced by "Tom".

For more usage, please see the document of Vuejs: https://vuejs.org/v2/guide/components-slots.html
