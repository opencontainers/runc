// +build linux

package configs

import (
	"testing"
	"encoding/json"
)

func TestSwappinessType(t *testing.T) {

	test := []byte(`{"swappiness":-1}`)
	test2 := []byte(`{"swappiness":29}`)

	x := struct{ AA Swappiness `json:"swappiness"` }{AA: 6, }
	// over
	err := json.Unmarshal(test, &x)
	if err != nil {
		t.Fatalf("Can't unmarshal %v", err)
	}
	// default set to 60 in Swappiness when -1 used
	if x.AA != 60 {
		t.Fatalf("Expected linux swappiness default of 60 when -1 used.  Got %v", x)
	}
	// over
	err = json.Unmarshal(test2, &x)
	if err != nil {
		t.Fatalf("Can't unmarshal %v", err)
	}
	if x.AA != 29 {
		t.Fatalf("Normal Uint64 function failed %v ",x)
	}
	// now marshal back out.  Do not emit
	bb, marshalErr := json.Marshal(x)
	if marshalErr != nil {
		t.Fatalf("Marshal err %v swappiness failed",marshalErr)
	}
	if string(bb) != string(test2) {
		t.Fatalf("Marshal swappiness failed got >%s< want >%s<",string(bb),string(test2))

	}
}

