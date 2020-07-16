package configs_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestUnmarshalIntelRDT(t *testing.T) {
	testCases := []struct {
		JSON     string
		Expected configs.IntelRdt
	}{
		{
			"{\"enableMBM\": true}",
			configs.IntelRdt{EnableMBM: true, EnableCMT: false},
		},
		{
			"{\"enableMBM\": true,\"enableCMT\": false}",
			configs.IntelRdt{EnableMBM: true, EnableCMT: false},
		},
		{
			"{\"enableMBM\": false,\"enableCMT\": true}",
			configs.IntelRdt{EnableMBM: false, EnableCMT: true},
		},
	}

	for _, tc := range testCases {
		got := configs.IntelRdt{}

		err := json.Unmarshal([]byte(tc.JSON), &got)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(tc.Expected, got) {
			t.Errorf("expected unmarshalled IntelRDT config %+v, got %+v", tc.Expected, got)
		}
	}
}
