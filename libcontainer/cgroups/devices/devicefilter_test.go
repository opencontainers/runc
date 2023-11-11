package devices

import (
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

func hash(s, comm string) string {
	var res []string
	for _, l := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, comm) {
			continue
		}
		res = append(res, trimmed)
	}
	return strings.Join(res, "\n")
}

func testDeviceFilter(t testing.TB, devices []*devices.Rule, expectedStr string) {
	insts, _, err := deviceFilter(devices)
	if err != nil {
		t.Fatalf("%s: %v (devices: %+v)", t.Name(), err, devices)
	}
	s := insts.String()
	if expectedStr != "" {
		hashed := hash(s, "//")
		expectedHashed := hash(expectedStr, "//")
		if expectedHashed != hashed {
			t.Fatalf("expected:\n%q\ngot\n%q", expectedHashed, hashed)
		}
	}
}

func TestDeviceFilter_Nil(t *testing.T) {
	expected := `
// load parameters into registers
        0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
        1: AndImm32 dst: r2 imm: 65535
        2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        3: RShImm32 dst: r3 imm: 16
        4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject)
        6: MovImm32 dst: r0 imm: 0
        7: Exit
	`
	testDeviceFilter(t, nil, expected)
}

func TestDeviceFilter_BuiltInAllowList(t *testing.T) {
	expected := `
// load parameters into registers
        0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
        1: AndImm32 dst: r2 imm: 65535
        2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        3: RShImm32 dst: r3 imm: 16
        4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// (b, wildcard, wildcard, m, true)
        6: JNEImm dst: r2 off: -1 imm: 1 <block-1>
        7: MovReg32 dst: r1 src: r3
        8: AndImm32 dst: r1 imm: 1
        9: JNEReg dst: r1 off: -1 src: r3 <block-1>
        10: MovImm32 dst: r0 imm: 1
        11: Exit
block-1:
// (c, wildcard, wildcard, m, true)
        12: JNEImm dst: r2 off: -1 imm: 2 <block-2>
        13: MovReg32 dst: r1 src: r3
        14: AndImm32 dst: r1 imm: 1
        15: JNEReg dst: r1 off: -1 src: r3 <block-2>
        16: MovImm32 dst: r0 imm: 1
        17: Exit
block-2:
        18: JNEImm dst: r2 off: -1 imm: 2 <block-3>
        19: JNEImm dst: r4 off: -1 imm: 1 <block-3>
        20: JNEImm dst: r5 off: -1 imm: 3 <block-3>
        21: MovImm32 dst: r0 imm: 1
        22: Exit
block-3:
        23: JNEImm dst: r2 off: -1 imm: 2 <block-4>
        24: JNEImm dst: r4 off: -1 imm: 1 <block-4>
        25: JNEImm dst: r5 off: -1 imm: 5 <block-4>
        26: MovImm32 dst: r0 imm: 1
        27: Exit
block-4:
        28: JNEImm dst: r2 off: -1 imm: 2 <block-5>
        29: JNEImm dst: r4 off: -1 imm: 1 <block-5>
        30: JNEImm dst: r5 off: -1 imm: 7 <block-5>
        31: MovImm32 dst: r0 imm: 1
        32: Exit
block-5:
        33: JNEImm dst: r2 off: -1 imm: 2 <block-6>
        34: JNEImm dst: r4 off: -1 imm: 1 <block-6>
        35: JNEImm dst: r5 off: -1 imm: 8 <block-6>
        36: MovImm32 dst: r0 imm: 1
        37: Exit
block-6:
        38: JNEImm dst: r2 off: -1 imm: 2 <block-7>
        39: JNEImm dst: r4 off: -1 imm: 1 <block-7>
        40: JNEImm dst: r5 off: -1 imm: 9 <block-7>
        41: MovImm32 dst: r0 imm: 1
        42: Exit
block-7:
        43: JNEImm dst: r2 off: -1 imm: 2 <block-8>
        44: JNEImm dst: r4 off: -1 imm: 5 <block-8>
        45: JNEImm dst: r5 off: -1 imm: 0 <block-8>
        46: MovImm32 dst: r0 imm: 1
        47: Exit
block-8:
        48: JNEImm dst: r2 off: -1 imm: 2 <block-9>
        49: JNEImm dst: r4 off: -1 imm: 5 <block-9>
        50: JNEImm dst: r5 off: -1 imm: 2 <block-9>
        51: MovImm32 dst: r0 imm: 1
        52: Exit
block-9:
// /dev/pts (c, 136, wildcard, rwm, true)
        53: JNEImm dst: r2 off: -1 imm: 2 <block-10>
        54: JNEImm dst: r4 off: -1 imm: 136 <block-10>
        55: MovImm32 dst: r0 imm: 1
        56: Exit
block-10:
        57: MovImm32 dst: r0 imm: 0
        58: Exit
`
	var devices []*devices.Rule
	for _, device := range specconv.AllowedDevices {
		devices = append(devices, &device.Rule)
	}
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_Privileged(t *testing.T) {
	devices := []*devices.Rule{
		{
			Type:        'a',
			Major:       -1,
			Minor:       -1,
			Permissions: "rwm",
			Allow:       true,
		},
	}
	expected := `
// load parameters into registers
        0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
        1: AndImm32 dst: r2 imm: 65535
        2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        3: RShImm32 dst: r3 imm: 16
        4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 1 (accept)
        6: MovImm32 dst: r0 imm: 1
        7: Exit
	`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_PrivilegedExceptSingleDevice(t *testing.T) {
	devices := []*devices.Rule{
		{
			Type:        'a',
			Major:       -1,
			Minor:       -1,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        'b',
			Major:       8,
			Minor:       0,
			Permissions: "rwm",
			Allow:       false,
		},
	}
	expected := `
// load parameters into registers
         0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
         1: AndImm32 dst: r2 imm: 65535
         2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         3: RShImm32 dst: r3 imm: 16
         4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject) if type==b && major == 8 && minor == 0
         6: JNEImm dst: r2 off: -1 imm: 1 <block-1>
         7: JNEImm dst: r4 off: -1 imm: 8 <block-1>
         8: JNEImm dst: r5 off: -1 imm: 0 <block-1>
         9: MovImm32 dst: r0 imm: 0
        10: Exit
block-1:
// return 1 (accept)
        11: MovImm32 dst: r0 imm: 1
        12: Exit
`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_Weird(t *testing.T) {
	devices := []*devices.Rule{
		{
			Type:        'b',
			Major:       8,
			Minor:       1,
			Permissions: "rwm",
			Allow:       false,
		},
		{
			Type:        'a',
			Major:       -1,
			Minor:       -1,
			Permissions: "rwm",
			Allow:       true,
		},
		{
			Type:        'b',
			Major:       8,
			Minor:       2,
			Permissions: "rwm",
			Allow:       false,
		},
	}
	// 8/1 is allowed, 8/2 is not allowed.
	// This conforms to runc v1.0.0-rc.9 (cgroup1) behavior.
	expected := `
// load parameters into registers
         0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
         1: AndImm32 dst: r2 imm: 65535
         2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         3: RShImm32 dst: r3 imm: 16
         4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject) if type==b && major == 8 && minor == 2
         6: JNEImm dst: r2 off: -1 imm: 1 <block-1>
         7: JNEImm dst: r4 off: -1 imm: 8 <block-1>
         8: JNEImm dst: r5 off: -1 imm: 2 <block-1>
         9: MovImm32 dst: r0 imm: 0
        10: Exit
block-1:
// return 1 (accept)
        11: MovImm32 dst: r0 imm: 1
        12: Exit
`
	testDeviceFilter(t, devices, expected)
}
