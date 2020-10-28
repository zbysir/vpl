package dev

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"
)

// 测试map的内存占用
//  Alloc = 124008.7578125	TotalAlloc = 166458.078125	Sys = 176679.3671875	 NumGC = 6
func TestMap(t *testing.T) {
	a := map[string]interface{}{
		"a": 1,
		"b": 1,
		"c": 1,
		"d": 1,
	}
	d := map[string]interface{}{}
	for i := 0; i < 1000000; i++ {
		d[strconv.FormatInt(int64(i), 10)] = a
	}

	(d["1"].(map[string]interface{}))["a"] = 2

	// 修改的是同一个map, 不存在复制的情况
	t.Logf("%+v", d["1"])
	t.Logf("%+v", d["2"])

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	kb := 1024.0

	logstr := fmt.Sprintf("\nAlloc = %v\tTotalAlloc = %v\tSys = %v\t NumGC = %v\n\n", float64(m.Alloc)/kb, float64(m.TotalAlloc)/kb, float64(m.Sys)/kb, m.NumGC)
	t.Log(logstr)
}

type A struct {
	a int64
	b int
	c int
	d int
}

// Alloc = 96430.1640625	TotalAlloc = 128611.578125	Sys = 129657.9921875	 NumGC = 6
// 如果使用对象, 存放值,
func TestStruct(t *testing.T) {
	a := A{
		a: 1,
		b: 1,
		c: 1,
		d: 1,
	}

	// 如果用interface接受参数. 内存占用会更高(比用指针高1/4)
	// 124011.0703125	TotalAlloc = 166424.15625	Sys = 176423.3671875	 NumGC = 6
	// d := map[string]interface{}{}
	//
	// 用值, 会产生copy, 内存最多
	// 178689.0078125	TotalAlloc = 240875.03125	Sys = 239845.2421875	 NumGC = 7
	// d := map[string]A{}
	//
	// 如果直接用指针类型, 内存最少
	// 96430.1640625	TotalAlloc = 128611.578125	Sys = 129657.9921875	 NumGC = 6
	d := map[string]*A{}

	for i := 0; i < 1000000; i++ {
		d[strconv.FormatInt(int64(i), 10)] = &a
	}

	t.Logf("%+v", d["1"])
	t.Logf("%+v", d["2"])

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	kb := 1024.0

	logstr := fmt.Sprintf("\nAlloc = %v\tTotalAlloc = %v\tSys = %v\t NumGC = %v\n\n", float64(m.Alloc)/kb, float64(m.TotalAlloc)/kb, float64(m.Sys)/kb, m.NumGC)
	t.Log(logstr)
}
