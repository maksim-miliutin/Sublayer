package winapi

import (
	"errors"
	"testing"
)

func TestLoadFindsFunctionsOfASystemDLL(t *testing.T) {
	var create, draw uintptr

	err := Load("user32.dll", Proc{"CreateWindowExW", &create}, Proc{"DrawTextW", &draw})
	if err != nil {
		t.Fatal(err)
	}

	if create == 0 || draw == 0 {
		t.Errorf("got addresses %#x and %#x, want both found", create, draw)
	}
}

func TestLoadRefusesAFunctionTheDLLLacks(t *testing.T) {
	var none uintptr

	err := Load("user32.dll", Proc{"NoSuchFunctionAnywhere", &none})
	if !errors.Is(err, ErrMissing) {
		t.Errorf("got %v, want ErrMissing", err)
	}
}

func TestLoadRefusesADLLThatIsNotInSystem32(t *testing.T) {
	err := Load("sublayer-no-such-library.dll")
	if !errors.Is(err, ErrMissing) {
		t.Errorf("got %v, want ErrMissing", err)
	}
}
