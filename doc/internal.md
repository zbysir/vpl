# Vpl Internals

## Optimize

### Preprocessing template to Reduce runtime consumption.
Vpl parse `Vue template` and `JS expression` at build time to Reduce runtime consumption.

### Optimize static nodes as strings.
Template like this
```vue
<ul>
 <li>begin</li>
 <li>{{txt}}</li>
 <li>end</li>
</ul>
```
It will be optimized to 3 statements:
```
"<ul><li>begin</li><li>"
{{txt}}
"</li><li>end</li></ul>"
```
This will optimize performance, especially when there are many nodes.

### Slot does not generate closures if unnecessary

## With Go features
Let's add some go features to vpl.

### Parallel
The advantage of go is concurrency, can vpl use it?

YES! Use the `<parallel>` component.

Please refer to [syntax-parallel](./syntax.md#parallel)
