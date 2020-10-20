package test

import (
	"context"
	"fmt"
	"github.com/zbysir/vpl"
	"github.com/zbysir/vpl/internal/compiler"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

// 测试parallel组件
func TestParallel(t *testing.T) {
	cases := []Cases{
		{
			// 测试方法调用
			Name:           "parallel",
			IndexTpl:       "",
			IndexComponent: `main`,
			Tpl: []struct {
				Name string
				Txt  string
			}{{
				Name: "main",
				Txt: `
<!-- 总共花费6s -->

<div>
	<!-- 总共花费5s -->
	{{ sleep(3) }} 
	<div>
		<!-- 总共花费2s -->
		<parallel>
			<div>
				{{ sleep(1) }} 
			</div>
		</parallel>
		<parallel>
			<div>
				{{ sleep(2) }} 
			</div>
		</parallel>
	</div>
</div>

<div>
	<!-- 总共花费3s -->
	<parallel>
		<div> {{sleep(3)}} </div>				
	</parallel>
	<parallel>
		<div> {{sleep(1)}} </div>
	</parallel>
</div>`,
			}},
			Output: "output/%s.html",
			Checker: func(html string) error {

				return nil
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			vue := vpl.New()
			t.Logf("compile....")

			for _, tp := range c.Tpl {
				err := vue.ComponentTxt(tp.Name, tp.Txt)
				if err != nil {
					t.Fatal(err)
				}
			}

			t.Logf("run....")

			props := compiler.NewPropsR()
			props.AppendMap(map[string]interface{}{
				"sleep": func(r *vpl.Vpl, args ...interface{}) interface{} {
					ti := interfaceToFloat(args[0])
					time.Sleep(time.Duration(ti) * time.Second)
					return fmt.Sprintf("sleep %+v", ti)
				},
			})
			var html string
			var err error
			if c.IndexComponent != "" {
				html, err = vue.RenderComponent(c.IndexComponent, &vpl.RenderParam{
					Global: nil,
					Ctx:    context.Background(),
					PropsR: props,
				})
			} else {
				html, err = vue.RenderTpl(c.IndexTpl, &vpl.RenderParam{
					Global: nil,
					Ctx:    context.Background(),
					PropsR: props,
				})
			}

			if err != nil {
				t.Fatal(err)
			}

			if c.Checker != nil {
				err = c.Checker(html)
				if err != nil {
					t.Fatal(err)
				}
			}
			ioutil.WriteFile(fmt.Sprintf(c.Output, c.Name), []byte(html), os.ModePerm)

			t.Logf("%s", html)
		})
	}
}

func interfaceToFloat(s interface{}) (d float64) {
	if s == nil {
		return 0
	}
	switch a := s.(type) {
	case int:
		return float64(a)
	case int8:
		return float64(a)
	case int16:
		return float64(a)
	case int32:
		return float64(a)
	case int64:
		return float64(a)
	case float32:
		return float64(a)
	case float64:
		return a
	default:
		return 0
	}
}
