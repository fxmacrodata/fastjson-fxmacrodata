package main

import (
	"fmt"
	fxmd "github.com/fxmacrodata/fastjson-fxmacrodata"
)

func main() {
	// Fictional fixture, not official observations or real market data.
	doc, err := fxmd.ParseJSON([]byte(`{"data":[{"value":9007199254740993.0000,"revision":1,"unit":"fictional"}],"publication_time_status":"unknown"}`))
	if err != nil {
		panic(err)
	}
	row, err := doc.NativeAt("/data/0")
	if err != nil {
		panic(err)
	}
	fmt.Println(row.Get("value").String())
	fmt.Println(string(row.GetStringBytes("unit")))
}
