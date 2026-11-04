package fxmdjson

import (
	"bytes"
	"encoding/json"
	"github.com/valyala/fastjson"
	"strings"
	"sync"
	"testing"
)

func document(t *testing.T, raw string) *Document {
	t.Helper()
	d, e := ParseJSON([]byte(raw))
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestCompleteNativeViewAndExactNumbers(t *testing.T) {
	raw := ` {"data":[{"value":9007199254740993.0000,"revision":1,"clock":null,"unknown":{"x":1e+03}}],"next_cursor":"fictional","publication_time_status":"unknown"} `
	d := document(t, raw)
	v, e := d.Native()
	if e != nil {
		t.Fatal(e)
	}
	if v.Get("data", "0", "value").String() != "9007199254740993.0000" || v.Get("data", "0", "unknown", "x").String() != "1e+03" {
		t.Fatal("native numbers rounded")
	}
	if v.Get("data", "0", "clock") == nil || v.Get("data", "0", "clock").Type() != fastjson.TypeNull || v.Get("absent") != nil {
		t.Fatal("null/missing distinction lost")
	}
	if string(v.GetStringBytes("next_cursor")) != "fictional" || string(v.GetStringBytes("publication_time_status")) != "unknown" {
		t.Fatal("envelope lost")
	}
	if string(d.Raw()) != raw {
		t.Fatal("original changed")
	}
	for _, s := range []string{`0`, `-0`, `1.2300`, `1e+03`, `9007199254740993`, `1234567890123456789012345678901234567890`} {
		d := document(t, s)
		n, e := d.Native()
		if e != nil || n.Type() != fastjson.TypeNumber || n.String() != s {
			t.Fatal("number representation", s, e)
		}
	}
}
func TestNativeCopiesHaveIndependentLifetimeAndMutability(t *testing.T) {
	raw := []byte(`{"data":[{"value":1.00,"revision":0}],"unit":"original"}`)
	saved := append([]byte(nil), raw...)
	d, e := ParseJSON(raw)
	if e != nil {
		t.Fatal(e)
	}
	raw[0] = 'x'
	a, e := d.Native()
	if e != nil {
		t.Fatal(e)
	}
	b, e := d.Native()
	if e != nil {
		t.Fatal(e)
	}
	var arena fastjson.Arena
	a.Get("data", "0").Set("value", arena.NewNumberString("2.00"))
	a.Set("unit", arena.NewString("derived"))
	if b.Get("data", "0", "value").String() != "1.00" || string(b.GetStringBytes("unit")) != "original" {
		t.Fatal("native copies alias")
	}
	selected, e := d.NativeAt("/data/0")
	if e != nil || selected.Get("value").String() != "1.00" {
		t.Fatal("selection alias", e)
	}
	if !bytes.Equal(d.Raw(), saved) {
		t.Fatal("native edit changed original")
	}
	copy := d.Raw()
	copy[0] = 'x'
	if !bytes.Equal(d.Raw(), saved) {
		t.Fatal("raw copy alias")
	}
	for i := 0; i < 30; i++ {
		if _, e := d.Native(); e != nil {
			t.Fatal(e)
		}
	}
	if b.Get("data", "0", "value").String() != "1.00" {
		t.Fatal("later parser invalidated view")
	}
}
func TestAllShapesFamiliesAndSDKModels(t *testing.T) {
	for _, family := range []string{"catalogue", "macro", "calendar", "predictions", "news", "fx", "sessions", "cot", "commodities", "seasonality", "rates", "curves", "financial-prices", "research", "reference", "elections"} {
		t.Run(family, func(t *testing.T) {
			raw := `{"data":[{"family":"` + family + `","value":1.2300,"revision":1,"period":"fixture","source":{"url":"https://example.invalid/fixture"},"flags":[null,true,false]}],"next_cursor":"fixture","metadata":{"status":"unknown"}}`
			d := document(t, raw)
			v, e := d.Native()
			if e != nil {
				t.Fatal(e)
			}
			encoded := v.MarshalTo(nil)
			for _, text := range []string{`1.2300`, `"revision":1`, `"source"`, `[null,true,false]`, `"next_cursor"`, `"metadata"`} {
				if !bytes.Contains(encoded, []byte(text)) {
					t.Fatal("field changed", text)
				}
			}
			if string(d.Raw()) != raw {
				t.Fatal("raw changed")
			}
		})
	}
	for _, raw := range []string{`null`, `true`, `false`, `"fixture"`, `[]`, `{}`, `1.2300`} {
		d := document(t, raw)
		v, e := d.Native()
		if e != nil || v.String() != raw {
			t.Fatal("root shape", raw, e)
		}
	}
	model := struct {
		Rows []json.Number `json:"rows"`
	}{[]json.Number{"9007199254740993", "1.2300"}}
	d, e := Parse(model)
	if e != nil {
		t.Fatal(e)
	}
	v, e := d.Native()
	if e != nil || v.Get("rows", "0").String() != "9007199254740993" {
		t.Fatal("model", e)
	}
	if _, e := Parse(make(chan int)); e == nil {
		t.Fatal("nonserializable model")
	}
}
func TestPointerUnicodeErrorsAndBounds(t *testing.T) {
	d := document(t, `{"a/b":{"~":[{"\u6771\u4eac":"\u00e9"}]},"":null,"list":[0]}`)
	v, e := d.NativeAt("/a~1b/~0/0")
	if e != nil || string(v.GetStringBytes("\u6771\u4eac")) != "\u00e9" {
		t.Fatal("Unicode selection", e)
	}
	found, e := d.Exists("/")
	if e != nil || !found {
		t.Fatal("empty key null")
	}
	n, e := d.NativeAt("/")
	if e != nil || n.Type() != fastjson.TypeNull {
		t.Fatal("null selection", e)
	}
	for _, path := range []string{"bad", "/a~2b", "/absent", "/list/01", "/list/-", "/" + strings.Repeat("x", 4097), string([]byte{47, 255})} {
		if _, e := d.NativeAt(path); e == nil {
			t.Fatal("invalid/missing pointer", path)
		}
	}
	for _, raw := range [][]byte{nil, []byte(`{"x":1,"x":2}`), []byte(`{"\u0078":1,"x":2}`), []byte(`[] []`), []byte(`{"n":NaN}`), {0xff}, []byte(strings.Repeat("[", MaxJSONDepth+2) + "0" + strings.Repeat("]", MaxJSONDepth+2)), []byte(strings.Repeat(" ", MaxResponseBytes+1)), []byte("[" + strings.Repeat("null,", MaxJSONValues) + "null]")} {
		if _, e := ParseJSON(raw); e == nil {
			t.Fatal("malformed/unbounded response accepted")
		}
	}
	big := document(t, `"`+strings.Repeat("<", MaxViewBytes/6+1)+`"`)
	if _, e := big.Native(); e == nil {
		t.Fatal("expanded native view exceeded bounds")
	}
	var nilDoc *Document
	if _, e := nilDoc.Native(); e == nil {
		t.Fatal("nil receiver")
	}
}
func TestConcurrentNativeViews(t *testing.T) {
	d := document(t, `{"rows":[{"value":1.2300,"clock":null}]}`)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				v, e := d.Native()
				if e != nil {
					t.Error(e)
					continue
				}
				var arena fastjson.Arena
				v.Set("derived", arena.NewTrue())
				if v.Get("rows", "0", "value").String() != "1.2300" {
					t.Error("concurrent value")
				}
			}
		}()
	}
	wg.Wait()
	v, e := d.Native()
	if e != nil || v.Get("derived") != nil {
		t.Fatal("shared mutation", e)
	}
}

func TestAllNativePrimitiveNodesAreIndependentlyOwned(t *testing.T) {
	d := document(t, `{"values":[null,true,false,1.2300,"fixture"],"nested":{"n":null}}`)
	a, e := d.Native()
	if e != nil {
		t.Fatal(e)
	}
	b, e := d.Native()
	if e != nil {
		t.Fatal(e)
	}
	var arena fastjson.Arena
	for _, path := range [][]string{{"values", "0"}, {"values", "1"}, {"values", "2"}, {"values", "3"}, {"values", "4"}, {"nested", "n"}} {
		*a.Get(path...) = *arena.NewString("derived")
	}
	expected := []string{"null", "true", "false", "1.2300", `"fixture"`}
	for i, text := range expected {
		if b.Get("values", string(rune('0'+i))).String() != text {
			t.Fatal("primitive alias", i)
		}
	}
	c, e := d.Native()
	if e != nil || c.Get("nested", "n").Type() != fastjson.TypeNull {
		t.Fatal("native edits affected later parse", e)
	}
	for _, raw := range []string{"null", "true", "false"} {
		doc := document(t, raw)
		first, _ := doc.Native()
		second, _ := doc.Native()
		*first = *arena.NewString("derived")
		if second.String() != raw {
			t.Fatal("root primitive alias")
		}
	}
}
