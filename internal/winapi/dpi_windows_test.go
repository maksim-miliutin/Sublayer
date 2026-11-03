package winapi

import "testing"

func TestAwareOfDPIFindsWhatItCalls(t *testing.T) {
	if err := AwareOfDPI(); err != nil {
		t.Error(err)
	}
}
