package templates

import (
	"context"
	"encoding/json"
	"github.com/CloudyKit/jet"
	"github.com/valyala/quicktemplate"
	"github.com/zbysir/vpl"
	"log"
	"strings"
	"testing"
)

// cd bench
// go test -bench=. -benchmem
func BenchmarkQuickTemplate1(b *testing.B) {
	benchmarkQuickTemplate(b, 1)
}

func BenchmarkQuickTemplate10(b *testing.B) {
	benchmarkQuickTemplate(b, 10)
}

func BenchmarkQuickTemplate100(b *testing.B) {
	benchmarkQuickTemplate(b, 100)
}
func BenchmarkVpl1(b *testing.B) {
	benchmarkVpl(b, 1)
}

func BenchmarkVpl10(b *testing.B) {
	benchmarkVpl(b, 10)
}

func BenchmarkVpl100(b *testing.B) {
	benchmarkVpl(b, 100)
}

func BenchmarkJet1(b *testing.B) {
	benchmarkJet(b, 1)
}

func BenchmarkJet10(b *testing.B) {
	benchmarkJet(b, 10)
}

func BenchmarkJet100(b *testing.B) {
	benchmarkJet(b, 100)
}

func benchmarkQuickTemplate(b *testing.B, rowsCount int) {
	b.ReportAllocs()
	rows := getBenchRows(rowsCount)
	b.RunParallel(func(pb *testing.PB) {
		bb := quicktemplate.AcquireByteBuffer()
		for pb.Next() {
			WriteBenchPage(bb, rows)
			bb.Reset()
		}
		quicktemplate.ReleaseByteBuffer(bb)
	})
}

func benchmarkVpl(b *testing.B, rowsCount int) {
	rows := getBenchRows(rowsCount)
	vue := vpl.New()
	err := vue.ComponentTxt("main", `<html>
	<head><title>test</title></head>
	<body>
		<ul>
			<template v-for="item in rows">
				<li v-if="item.Print">ID={{item.ID}}, Message={{item.Message}}</li>
			</template>
		</ul>
	</body>
</html>`)
	if err != nil {
		b.Fatal(err)
	}
	var ii interface{}
	bs, _ := json.Marshal(rows)
	json.Unmarshal(bs, &ii)
	props := vpl.NewProps()
	props.AppendMap(map[string]interface{}{
		"rows": ii,
	})

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := vue.RenderComponent("main", &vpl.RenderParam{
				Global: nil,
				Ctx:    context.Background(),
				Props:  props,
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func benchmarkJet(b *testing.B, rowsCount int) {
	var views = jet.NewHTMLSet("./jetviews")

	view, err := views.GetTemplate("bench.jet")
	if err != nil {
		log.Println("Unexpected template err:", err.Error())
	}

	rows := getBenchRows(rowsCount)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var s strings.Builder
			view.Execute(&s, nil, rows)
		}
	})
}

func TestJet(t *testing.T) {
	var views = jet.NewHTMLSet("./jetviews")

	view, err := views.GetTemplate("bench.jet")
	if err != nil {
		t.Fatal(err)
	}

	rows := getBenchRows(10)
	var s strings.Builder
	err = view.Execute(&s, nil, rows)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%s", s.String())
}
