package test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zbysir/vpl"
	"github.com/zbysir/vpl/internal/compiler"
	"sync"
	"testing"
)

type BenchRow struct {
	ID      int
	Message string
	Print   bool
}

func getBenchRows(n int) []BenchRow {
	rows := make([]BenchRow, n)
	for i := 0; i < n; i++ {
		rows[i] = BenchRow{
			ID:      i,
			Message: fmt.Sprintf("message %d", i),
			Print:   (i & 1) == 0,
			//Print:   true,
		}
	}
	return rows
}

// 测试并发渲染
func TestGoParallel(t *testing.T) {
	rows := getBenchRows(10)
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
		t.Fatal(err)
	}
	var ii interface{}
	bs, _ := json.Marshal(rows)
	json.Unmarshal(bs, &ii)
	props := compiler.NewProps()
	props.AppendMap(map[string]interface{}{
		"rows": ii,
	})

	var wg sync.WaitGroup

	for i := 0; i < 400; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ht, err := vue.RenderComponent("main", &vpl.RenderParam{
				Global: nil,
				Ctx:    context.Background(),
				Props:  props,
			})
			if err != nil {
				t.Fatal(err)
			}

			if ht != "<html><head><title>test</title></head><body><ul><li>ID=0, Message=message 0</li><li>ID=2, Message=message 2</li><li>ID=4, Message=message 4</li><li>ID=6, Message=message 6</li><li>ID=8, Message=message 8</li></ul></body></html>" {
				t.Fatal(ht)
			}

			//t.Logf("%v %s", rows[0].ID, ht)
		}()
	}

	wg.Wait()
}
