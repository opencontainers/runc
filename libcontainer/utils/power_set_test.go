package utils

import (
	"reflect"
	"testing"
)

func testBitPowerSet(t *testing.T, mask uint64, wanted []uint64) {
	var got []uint64
	if err := BitPowerSet(mask, func(value uint64) error {
		got = append(got, value)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !reflect.DeepEqual(got, wanted) {
		t.Errorf("mismatch on BitPowerSet(%#x): wanted %#v, got %#v", mask, wanted, got)
	}
}

func TestBitPowerSet(t *testing.T) {
	testBitPowerSet(t, 0xA00, []uint64{0x0, 0x200, 0x800, 0xA00})
	testBitPowerSet(t, 0x111, []uint64{0x000, 0x001, 0x010, 0x011, 0x100, 0x101, 0x110, 0x111})
	testBitPowerSet(t, 0x10000, []uint64{0x00000, 0x10000})
}
