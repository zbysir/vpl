package compiler

import "testing"

func TestGetClassFromProps(t *testing.T) {
	c := getClassFromProps([]interface{}{
		map[string]interface{}{
			"t": true,
		},
		"d",
		"c",
	})

	// t d c
	t.Logf("%+v", c)
}
