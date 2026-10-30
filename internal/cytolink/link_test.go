package cytolink

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

func TestExactCSV(t *testing.T) {
	text := "\ufeffa,b,c\r\n\"quoted\r\nline\",\"say \"\"hello\"\"\",9007199254740993\r\nempty,,0.1234567890123456789\r\n"
	got, e := CSV(text)
	if e != nil || !reflect.DeepEqual(got, [][]string{{"a", "b", "c"}, {"quoted\r\nline", `say "hello"`, "9007199254740993"}, {"empty", "", "0.1234567890123456789"}}) {
		t.Fatal(got, e)
	}
	for _, s := range []string{"a,a\n1,2\n", "a,b\n1\n", "a\n\"unclosed", "a\nx\"y", "a\n\"x\"bad", "a\nx\ry", "a\n\x00", "a\n\xff", "a\n", "a\nb\n\n", strings.Repeat("a,", 128) + "b\nx\n"} {
		if _, e := CSV(s); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	if _, e := CSV("a\n" + strings.Repeat("b\n", MaxRows+1)); e == nil {
		t.Fatal("row bound")
	}
}
func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	table := &rep.Table{Samples: []rep.Sample{{Name: "A"}}, Cell: []uint32{}}
	_, e := Link(ctx, table, 0, "well,row,column,event,population\nA1,1,1,1,x\n", "well,cell_id\nA1,cell\n")
	if e != context.Canceled {
		t.Fatal(e)
	}
}
