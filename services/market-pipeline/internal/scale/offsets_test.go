package scale

import (
	"strings"
	"testing"
)

func TestOffsetWindow(t *testing.T) {
	partitions := []int32{0, 1, 2}
	window, err := NewOffsetWindow(`{"0":10,"1":20,"2":30}`, `{"0":12,"1":23,"2":35}`, partitions, 10)
	start, end, boundsErr := window.Bounds(2, 0, 40)
	if err != nil || boundsErr != nil || start != 30 || end != 35 {
		t.Fatalf("offset window = %d:%d, %v, %v", start, end, err, boundsErr)
	}
	legacy, err := NewOffsetWindow("", "", partitions, 10)
	start, end, boundsErr = legacy.Bounds(0, 4, 9)
	if err != nil || boundsErr != nil || start != 4 || end != 9 {
		t.Fatalf("legacy offset window = %d:%d, %v, %v", start, end, err, boundsErr)
	}

	for _, test := range []struct {
		name  string
		start string
		end   string
		want  string
	}{
		{name: "one sided", start: `{}`, want: "provided together"},
		{name: "start JSON", start: `{`, end: `{}`, want: "decode start"},
		{name: "end JSON", start: `{}`, end: `{`, want: "decode end"},
		{name: "missing partition", start: `{"0":0,"1":0}`, end: `{"0":1,"1":1}`, want: "partition 2"},
		{name: "negative", start: `{"0":-1,"1":0,"2":0}`, end: `{"0":1,"1":1,"2":1}`, want: "partition 0"},
		{name: "reversed", start: `{"0":2,"1":0,"2":0}`, end: `{"0":1,"1":1,"2":1}`, want: "partition 0"},
		{name: "extra partition", start: `{"0":0,"1":0,"2":0,"3":0}`, end: `{"0":1,"1":1,"2":1,"3":1}`, want: "does not match"},
		{name: "wrong count", start: `{"0":0,"1":0,"2":0}`, end: `{"0":1,"1":1,"2":1}`, want: "contains 3 records"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewOffsetWindow(test.start, test.end, partitions, 10)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewOffsetWindow() error = %v, want %q", err, test.want)
			}
		})
	}

	for _, test := range []struct {
		name      string
		window    OffsetWindow
		partition int32
	}{
		{name: "missing", window: OffsetWindow{start: map[int32]int64{0: 1}, end: map[int32]int64{}}, partition: 0},
		{name: "before retention", window: window, partition: 0},
		{name: "after newest", window: window, partition: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			oldest, newest := int64(0), int64(100)
			if test.name == "before retention" {
				oldest = 11
			}
			if test.name == "after newest" {
				newest = 34
			}
			if _, _, err := test.window.Bounds(test.partition, oldest, newest); err == nil || !strings.Contains(err.Error(), "outside retained") {
				t.Fatalf("Bounds() error = %v", err)
			}
		})
	}
}
