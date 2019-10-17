package devicefilter

import (
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
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

func testDeviceFilter(t testing.TB, devices []*configs.Device, expectedStr string) {
	insts, _, err := DeviceFilter(devices)
	if err != nil {
		t.Fatalf("%s: %v (devices: %+v)", t.Name(), err, devices)
	}
	s := insts.String()
	t.Logf("%s: devices: %+v\n%s", t.Name(), devices, s)
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
        0: LdXMemH dst: r2 src: r1 off: 0 imm: 0
        1: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        2: RSh32Imm dst: r3 imm: 16
        3: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        4: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject)
        5: Mov32Imm dst: r0 imm: 0
        6: Exit
	`
	testDeviceFilter(t, nil, expected)
}

func TestDeviceFilter_BuiltInAllowList(t *testing.T) {
	expected := `
// load parameters into registers
         0: LdXMemH dst: r2 src: r1 off: 0 imm: 0
         1: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         2: RSh32Imm dst: r3 imm: 16
         3: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         4: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// tuntap (c, 10, 200, rwm, allow)
         5: JNEImm dst: r2 off: -1 imm: 2 <block-1>
         6: JNEImm dst: r4 off: -1 imm: 10 <block-1>
         7: JNEImm dst: r5 off: -1 imm: 200 <block-1>
         8: Mov32Imm dst: r0 imm: 1
         9: Exit
block-1:
        10: JNEImm dst: r2 off: -1 imm: 2 <block-2>
        11: JNEImm dst: r4 off: -1 imm: 5 <block-2>
        12: JNEImm dst: r5 off: -1 imm: 2 <block-2>
        13: Mov32Imm dst: r0 imm: 1
        14: Exit
block-2:
// /dev/pts (c, 136, wildcard, rwm, true)
        15: JNEImm dst: r2 off: -1 imm: 2 <block-3>
        16: JNEImm dst: r4 off: -1 imm: 136 <block-3>
        17: Mov32Imm dst: r0 imm: 1
        18: Exit
block-3:
        19: JNEImm dst: r2 off: -1 imm: 2 <block-4>
        20: JNEImm dst: r4 off: -1 imm: 5 <block-4>
        21: JNEImm dst: r5 off: -1 imm: 1 <block-4>
        22: Mov32Imm dst: r0 imm: 1
        23: Exit
block-4:
        24: JNEImm dst: r2 off: -1 imm: 2 <block-5>
        25: JNEImm dst: r4 off: -1 imm: 1 <block-5>
        26: JNEImm dst: r5 off: -1 imm: 9 <block-5>
        27: Mov32Imm dst: r0 imm: 1
        28: Exit
block-5:
        29: JNEImm dst: r2 off: -1 imm: 2 <block-6>
        30: JNEImm dst: r4 off: -1 imm: 1 <block-6>
        31: JNEImm dst: r5 off: -1 imm: 5 <block-6>
        32: Mov32Imm dst: r0 imm: 1
        33: Exit
block-6:
        34: JNEImm dst: r2 off: -1 imm: 2 <block-7>
        35: JNEImm dst: r4 off: -1 imm: 5 <block-7>
        36: JNEImm dst: r5 off: -1 imm: 0 <block-7>
        37: Mov32Imm dst: r0 imm: 1
        38: Exit
block-7:
        39: JNEImm dst: r2 off: -1 imm: 2 <block-8>
        40: JNEImm dst: r4 off: -1 imm: 1 <block-8>
        41: JNEImm dst: r5 off: -1 imm: 7 <block-8>
        42: Mov32Imm dst: r0 imm: 1
        43: Exit
block-8:
        44: JNEImm dst: r2 off: -1 imm: 2 <block-9>
        45: JNEImm dst: r4 off: -1 imm: 1 <block-9>
        46: JNEImm dst: r5 off: -1 imm: 8 <block-9>
        47: Mov32Imm dst: r0 imm: 1
        48: Exit
block-9:
        49: JNEImm dst: r2 off: -1 imm: 2 <block-10>
        50: JNEImm dst: r4 off: -1 imm: 1 <block-10>
        51: JNEImm dst: r5 off: -1 imm: 3 <block-10>
        52: Mov32Imm dst: r0 imm: 1
        53: Exit
block-10:
// (b, wildcard, wildcard, m, true)
        54: JNEImm dst: r2 off: -1 imm: 1 <block-11>
        55: Mov32Reg dst: r1 src: r3
        56: And32Imm dst: r1 imm: 1
        57: JEqImm dst: r1 off: -1 imm: 0 <block-11>
        58: Mov32Imm dst: r0 imm: 1
        59: Exit
block-11:
// (c, wildcard, wildcard, m, true)
        60: JNEImm dst: r2 off: -1 imm: 2 <block-12>
        61: Mov32Reg dst: r1 src: r3
        62: And32Imm dst: r1 imm: 1
        63: JEqImm dst: r1 off: -1 imm: 0 <block-12>
        64: Mov32Imm dst: r0 imm: 1
        65: Exit
block-12:
        66: Mov32Imm dst: r0 imm: 0
        67: Exit
`
	testDeviceFilter(t, specconv.AllowedDevices, expected)
}

func TestDeviceFilter_Privileged(t *testing.T) {
	devices := []*configs.Device{
		{
			Type:        'a',
			Major:       -1,
			Minor:       -1,
			Permissions: "rwm",
			Allow:       true,
		},
	}
	expected :=
		`
// load parameters into registers
        0: LdXMemH dst: r2 src: r1 off: 0 imm: 0
        1: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        2: RSh32Imm dst: r3 imm: 16
        3: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        4: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 1 (accept)
        5: Mov32Imm dst: r0 imm: 1
        6: Exit
	`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_PrivilegedExceptSingleDevice(t *testing.T) {
	devices := []*configs.Device{
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
         0: LdXMemH dst: r2 src: r1 off: 0 imm: 0
         1: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         2: RSh32Imm dst: r3 imm: 16
         3: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         4: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject) if type==b && major == 8 && minor == 0
         5: JNEImm dst: r2 off: -1 imm: 1 <block-1>
         6: JNEImm dst: r4 off: -1 imm: 8 <block-1>
         7: JNEImm dst: r5 off: -1 imm: 0 <block-1>
         8: Mov32Imm dst: r0 imm: 0
         9: Exit
block-1:
// return 1 (accept)
        10: Mov32Imm dst: r0 imm: 1
        11: Exit
`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_Weird(t *testing.T) {
	devices := []*configs.Device{
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
         0: LdXMemH dst: r2 src: r1 off: 0 imm: 0
         1: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         2: RSh32Imm dst: r3 imm: 16
         3: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         4: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject) if type==b && major == 8 && minor == 2
         5: JNEImm dst: r2 off: -1 imm: 1 <block-1>
         6: JNEImm dst: r4 off: -1 imm: 8 <block-1>
         7: JNEImm dst: r5 off: -1 imm: 2 <block-1>
         8: Mov32Imm dst: r0 imm: 0
         9: Exit
block-1:
// return 1 (accept)
        10: Mov32Imm dst: r0 imm: 1
        11: Exit
`
	testDeviceFilter(t, devices, expected)
}
