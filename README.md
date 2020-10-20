# vpl
A tpl engin for golang, template syntax like vue.

方便使用模板引擎, 模板语法和vue类似, 不需要预编译成go语言.

和vue不同:
- 上层传递的 attr(id, style, class)等 不会自动作用在组件根节点, 
  所有从上传传递的参数需要在组件中自己处理.
  简化了逻辑, 并且原功能易导致混淆.
- js表达式只支持ES5, 为了性能, 请尽量简化js表达式, 或使用go func代替.

## Dependencies
- github.com/robertkrimen/otto
  
  解析js表达式, 为什么不同esbuild? esbuild额外强大, 会造成复杂度高. 写在模板中的js不会过于复杂, otto足矣.
- 