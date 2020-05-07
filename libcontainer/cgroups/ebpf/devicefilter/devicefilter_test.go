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

func testDeviceFilter(t testing.TB, devices []*configs.DeviceRule, expectedStr string) {
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
        0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
        1: And32Imm dst: r2 imm: 65535
        2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        3: RSh32Imm dst: r3 imm: 16
        4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject)
        6: Mov32Imm dst: r0 imm: 0
        7: Exit
	`
	testDeviceFilter(t, nil, expected)
}

func TestDeviceFilter_BuiltInAllowList(t *testing.T) {
	expected := `
// load parameters into registers
         0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
         1: And32Imm dst: r2 imm: 65535
         2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         3: RSh32Imm dst: r3 imm: 16
         4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// tuntap (c, 10, 200, rwm, allow)
         6: JNEImm dst: r2 off: -1 imm: 2 <block-1>
         7: JNEImm dst: r4 off: -1 imm: 10 <block-1>
         8: JNEImm dst: r5 off: -1 imm: 200 <block-1>
         9: Mov32Imm dst: r0 imm: 1
        10: Exit
block-1:
        11: JNEImm dst: r2 off: -1 imm: 2 <block-2>
        12: JNEImm dst: r4 off: -1 imm: 5 <block-2>
        13: JNEImm dst: r5 off: -1 imm: 2 <block-2>
        14: Mov32Imm dst: r0 imm: 1
        15: Exit
block-2:
// /dev/pts (c, 136, wildcard, rwm, true)
        16: JNEImm dst: r2 off: -1 imm: 2 <block-3>
        17: JNEImm dst: r4 off: -1 imm: 136 <block-3>
        18: Mov32Imm dst: r0 imm: 1
        19: Exit
block-3:
        20: JNEImm dst: r2 off: -1 imm: 2 <block-4>
        21: JNEImm dst: r4 off: -1 imm: 1 <block-4>
        22: JNEImm dst: r5 off: -1 imm: 9 <block-4>
        23: Mov32Imm dst: r0 imm: 1
        24: Exit
block-4:
        25: JNEImm dst: r2 off: -1 imm: 2 <block-5>
        26: JNEImm dst: r4 off: -1 imm: 1 <block-5>
        27: JNEImm dst: r5 off: -1 imm: 5 <block-5>
        28: Mov32Imm dst: r0 imm: 1
        29: Exit
block-5:
        30: JNEImm dst: r2 off: -1 imm: 2 <block-6>
        31: JNEImm dst: r4 off: -1 imm: 5 <block-6>
        32: JNEImm dst: r5 off: -1 imm: 0 <block-6>
        33: Mov32Imm dst: r0 imm: 1
        34: Exit
block-6:
        35: JNEImm dst: r2 off: -1 imm: 2 <block-7>
        36: JNEImm dst: r4 off: -1 imm: 1 <block-7>
        37: JNEImm dst: r5 off: -1 imm: 7 <block-7>
        38: Mov32Imm dst: r0 imm: 1
        39: Exit
block-7:
        40: JNEImm dst: r2 off: -1 imm: 2 <block-8>
        41: JNEImm dst: r4 off: -1 imm: 1 <block-8>
        42: JNEImm dst: r5 off: -1 imm: 8 <block-8>
        43: Mov32Imm dst: r0 imm: 1
        44: Exit
block-8:
        45: JNEImm dst: r2 off: -1 imm: 2 <block-9>
        46: JNEImm dst: r4 off: -1 imm: 1 <block-9>
        47: JNEImm dst: r5 off: -1 imm: 3 <block-9>
        48: Mov32Imm dst: r0 imm: 1
        49: Exit
block-9:
// (b, wildcard, wildcard, m, true)
        50: JNEImm dst: r2 off: -1 imm: 1 <block-10>
        51: Mov32Reg dst: r1 src: r3
        52: And32Imm dst: r1 imm: 1
        53: JEqImm dst: r1 off: -1 imm: 0 <block-10>
        54: Mov32Imm dst: r0 imm: 1
        55: Exit
block-10:
// (c, wildcard, wildcard, m, true)
        56: JNEImm dst: r2 off: -1 imm: 2 <block-11>
        57: Mov32Reg dst: r1 src: r3
        58: And32Imm dst: r1 imm: 1
        59: JEqImm dst: r1 off: -1 imm: 0 <block-11>
        60: Mov32Imm dst: r0 imm: 1
        61: Exit
block-11:
        62: Mov32Imm dst: r0 imm: 0
        63: Exit
`
	var devices []*configs.DeviceRule
	for _, device := range specconv.AllowedDevices {
		devices = append(devices, &device.DeviceRule)
	}
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_Privileged(t *testing.T) {
	devices := []*configs.DeviceRule{
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
        0: LdXMemW dst: r2 src: r1 off: 0 imm: 0
        1: And32Imm dst: r2 imm: 65535
        2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
        3: RSh32Imm dst: r3 imm: 16
        4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
        5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 1 (accept)
        6: Mov32Imm dst: r0 imm: 1
        7: Exit
	`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_PrivilegedExceptSingleDevice(t *testing.T) {
	devices := []*configs.DeviceRule{
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
         1: And32Imm dst: r2 imm: 65535
         2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         3: RSh32Imm dst: r3 imm: 16
         4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject) if type==b && major == 8 && minor == 0
         6: JNEImm dst: r2 off: -1 imm: 1 <block-1>
         7: JNEImm dst: r4 off: -1 imm: 8 <block-1>
         8: JNEImm dst: r5 off: -1 imm: 0 <block-1>
         9: Mov32Imm dst: r0 imm: 0
        10: Exit
block-1:
// return 1 (accept)
        11: Mov32Imm dst: r0 imm: 1
        12: Exit
`
	testDeviceFilter(t, devices, expected)
}

func TestDeviceFilter_Weird(t *testing.T) {
	devices := []*configs.DeviceRule{
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
         1: And32Imm dst: r2 imm: 65535
         2: LdXMemW dst: r3 src: r1 off: 0 imm: 0
         3: RSh32Imm dst: r3 imm: 16
         4: LdXMemW dst: r4 src: r1 off: 4 imm: 0
         5: LdXMemW dst: r5 src: r1 off: 8 imm: 0
block-0:
// return 0 (reject) if type==b && major == 8 && minor == 2
         6: JNEImm dst: r2 off: -1 imm: 1 <block-1>
         7: JNEImm dst: r4 off: -1 imm: 8 <block-1>
         8: JNEImm dst: r5 off: -1 imm: 2 <block-1>
         9: Mov32Imm dst: r0 imm: 0
        10: Exit
block-1:
// return 1 (accept)
        11: Mov32Imm dst: r0 imm: 1
        12: Exit
`
	testDeviceFilter(t, devices, expected)
}
