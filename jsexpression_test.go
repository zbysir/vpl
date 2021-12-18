package vpl

import (
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/robertkrimen/otto/parser"
	"github.com/zbysir/vpl/internal/lib/log"
)

type DataGet struct {
	data map[string]interface{}
}

func (d DataGet) Get(k string) interface{} {
	x, _, _ := ShouldLookInterface(d.data, k)
	return x
}

func TestRunJs(t *testing.T) {
	cases := []struct {
		Code  string
		Value interface{}
	}{

		{Code: "1+1", Value: 2},
		{Code: "a+1", Value: 2},
		{Code: "a-1", Value: 0},

		{Code: "-a", Value: -1},
		{Code: "!a", Value: false},
		{Code: "!!a", Value: true},
		{Code: "!0", Value: true},
		{Code: "!(a+1)", Value: false},
		{Code: "!(a-1)", Value: true},

		{Code: "2 > 1", Value: true},
		{Code: "2 >= 1", Value: true},
		{Code: "1 < 2", Value: true},
		{Code: "1 <= 2", Value: true},

		{Code: "info.sex", Value: 26},
		{Code: "info.sex+1", Value: 27},
		{Code: "info.sexkey", Value: "sex"},
		{Code: "info[info.sexkey]", Value: 26},

		{Code: "{'abc': 'abc'}['abc']", Value: "abc"},
		{Code: "{2: 3}['2']", Value: "3"},

		// getter
		{Code: "getter.a", Value: "1"},

		// call function
		{Code: "concat(1,2)", Value: "12"},
	}

	scope := NewScope(nil)
	scope.Value = map[string]interface{}{
		"a": 1,
		"info": map[string]interface{}{
			"sex":    26,
			"sexkey": "sex",
		},
		"getter": DataGet{
			data: map[string]interface{}{
				"a": 1,
			},
		},
		"concat": func(ctx *RenderCtx, args ...interface{}) interface{} {
			return fmt.Sprintf("%+v%+v", args[0], args[1])
		},
	}

	for _, c := range cases {
		p, err := parser.ParseFile(nil, "", "("+c.Code+")", 0)
		if err != nil {
			err = fmt.Errorf("GetAst err: %w, code:%s", err, c.Code)
			log.Warning(err)
			t.Fatal(err)
		}
		v, err := runJsExpression(p.Body[0], &RenderCtx{
			Scope: scope,
			Store: nil,
		})
		if err != nil {
			log.Warningf("runJsExpression err:%v", err)
			t.Fatal(err)
		}

		if fmt.Sprintf("%v", v) != fmt.Sprintf("%v", c.Value) {
			t.Fatal(fmt.Sprintf("code %s, want:%+v, get:%+v", c.Code, c.Value, v))
		}
	}

	t.Logf("OK")

}

func TestInterfaceToBool(t *testing.T) {
	var a int64 = 0
	if false != interfaceToBool(a) {
		t.Fatalf("%v , want false, but:%v", a, interfaceToBool(a))
	}

}

func TestGoja(t *testing.T) {
	vm := goja.New()
	v, err := vm.RunString("2 + 2")
	if err != nil {
		panic(err)
	}
	if num := v.Export().(int64); num != 4 {
		panic(num)
	}
}

func TestGojaEs6(t *testing.T) {
	vm := goja.New()
	// panic
	// not support es6 feature: Destructuring
	v, err := vm.RunString("var a = {a:1}; var b = {...a, c:1}; b")
	if err != nil {
		panic(err)
	}
	t.Log(v.Export())
}
