package ui

import (
	"reflect"
	"testing"
)

func TestACloneOfAFrameHoldsAllItSaysAndSharesNothing(t *testing.T) {
	f := Frame{Pairs: []Pair{{Original: "Oui.", Translation: "Да."}}, State: Recognizing, Hidden: true, Moving: true}

	fields := reflect.ValueOf(f)
	for i := range fields.NumField() {
		if fields.Field(i).IsZero() {
			t.Fatalf("the frame leaves %s at zero, so a clone that drops it would pass", fields.Type().Field(i).Name)
		}
	}

	c := f.clone()
	f.Pairs[0].Original = "Non."

	want := Frame{Pairs: []Pair{{Original: "Oui.", Translation: "Да."}}, State: Recognizing, Hidden: true, Moving: true}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("got %+v, want %+v", c, want)
	}
}
