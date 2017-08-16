package libcontainer

import (
	"encoding/json"
	"testing"
)

func TestUint64CompatType(t *testing.T) {

	test := []byte(`{"rc3":"9999999999999999999"}`)
	test2 := []byte(`{"rc3":9999999999999999998}`)

	x := struct {
		AA Uint64Compat `json:"rc3"`
	}{AA: 6}
	// over
	err := json.Unmarshal(test, &x)
	if err != nil {
		t.Fatalf("Can't unmarshal %v", err)
	}
	// default set to 60 in Swappiness when -1 used
	if x.AA != 9999999999999999999 {
		t.Fatalf("Expected linux 9999999999999999999.  Got %v", x)
	}
	// straight large number
	err = json.Unmarshal(test2, &x)
	if err != nil {
		t.Fatalf("Can't unmarshal %v", err)
	}
	if x.AA != 9999999999999999998 {
		t.Fatalf("Normal Uint64 function failed %v ", x)
	}
	// now marshal back out.  Do not emit
	bb, marshalErr := json.Marshal(x)
	if marshalErr != nil {
		t.Fatalf("Marshal err %v Uint64Compatfailed", marshalErr)
	}
	if string(bb) != string(test2) {
		t.Fatalf("Marshal Uint64Compat failed got >%s< want >%s<", string(bb), string(test2))

	}
}
