package configs

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestParseCPUMask(t *testing.T) {
	m, err := ParseCPUMask("")
	if err != nil || m != nil {
		t.Errorf(`ParseCPUMask(""): want nil, nil; got %v, %v`, m, err)
	}

	m, err = ParseCPUMask("all")
	if err != nil {
		t.Fatal(err)
	}
	if !m.All {
		t.Errorf(`ParseCPUMask("all"): want All, got %+v`, m)
	}
	if s := m.CPUSet(); s.Count() != MaxCPU {
		t.Errorf("all CPUSet: want %d CPUs, got %d", MaxCPU, s.Count())
	}

	m, err = ParseCPUMask("1,3")
	if err != nil {
		t.Fatal(err)
	}
	if m.All || m.Set.Count() != 2 || !m.Set.IsSet(1) || !m.Set.IsSet(3) {
		t.Errorf(`ParseCPUMask("1,3"): unexpected %+v`, m)
	}

	for _, in := range []string{"ALL", "all,1", "x", "3-1"} {
		if _, err := ParseCPUMask(in); err == nil {
			t.Errorf("ParseCPUMask(%q): want error, got nil", in)
		}
	}
}

func TestCPUAffinityJSON(t *testing.T) {
	// The format used by older runc versions (state.json).
	const legacy = `{"Initial":[10],"Final":null}`
	var aff CPUAffinity
	if err := json.Unmarshal([]byte(legacy), &aff); err != nil {
		t.Fatal(err)
	}
	if aff.Final != nil {
		t.Errorf("Final: want nil, got %+v", aff.Final)
	}
	// 10 is 0b1010, i.e. CPUs 1 and 3.
	if s := aff.Initial; s == nil || s.All || s.Set.Count() != 2 || !s.Set.IsSet(1) || !s.Set.IsSet(3) {
		t.Fatalf("Initial: unexpected %+v", aff.Initial)
	}
	out, err := json.Marshal(aff)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != legacy {
		t.Errorf("want %s, got %s", legacy, out)
	}

	// Round trip with "all".
	aff = CPUAffinity{Initial: &CPUMask{All: true}}
	out, err = json.Marshal(aff)
	if err != nil {
		t.Fatal(err)
	}
	const exp = `{"Initial":"all","Final":null}`
	if string(out) != exp {
		t.Errorf("want %s, got %s", exp, out)
	}
	aff = CPUAffinity{}
	if err := json.Unmarshal(out, &aff); err != nil {
		t.Fatal(err)
	}
	if aff.Initial == nil || !aff.Initial.All || aff.Final != nil {
		t.Errorf("unexpected %+v", aff)
	}
}

func TestConvertExecCPUAffinity(t *testing.T) {
	testCases := []struct {
		name           string
		p              *specs.Process
		initial, final string // "" for nil, "all", or a single CPU number.
		isNil, isErr   bool
	}{
		{name: "nil", p: nil, isNil: true},
		{name: "none", p: &specs.Process{}, isNil: true},
		{
			name:  "empty",
			p:     &specs.Process{CPUAffinity: &specs.CPUAffinity{CreateRuntime: "1"}},
			isNil: true,
		},
		{
			name:    "deprecated",
			p:       &specs.Process{ExecCPUAffinity: &specs.ExecCPUAffinity{Initial: "1", Final: "2"}}, //nolint:staticcheck // SA1019: testing deprecated field.
			initial: "1", final: "2",
		},
		{
			name: "new",
			p: &specs.Process{
				CPUAffinity: &specs.CPUAffinity{ExecRuntime: "all", ExecContainer: "3"},
			},
			initial: "all", final: "3",
		},
		{
			name: "both",
			p: &specs.Process{
				ExecCPUAffinity: &specs.ExecCPUAffinity{Initial: "1", Final: "2"}, //nolint:staticcheck // SA1019: testing deprecated field.
				CPUAffinity:     &specs.CPUAffinity{ExecContainer: "3"},
			},
			final: "3",
		},
		{
			name:  "bad",
			p:     &specs.Process{CPUAffinity: &specs.CPUAffinity{ExecRuntime: "x"}},
			isErr: true,
		},
	}

	check := func(t *testing.T, what string, m *CPUMask, exp string) {
		t.Helper()
		switch exp {
		case "":
			if m != nil {
				t.Errorf("%s: want nil, got %+v", what, m)
			}
		case "all":
			if m == nil || !m.All {
				t.Errorf("%s: want all, got %+v", what, m)
			}
		default:
			want, _ := ToCPUSet(exp)
			if m == nil || m.All || !slices.Equal(m.Set, want) {
				t.Errorf("%s: want %s, got %+v", what, exp, m)
			}
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aff, err := ConvertExecCPUAffinity(tc.p)
			if tc.isErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.isNil {
				if aff != nil {
					t.Fatalf("want nil, got %+v", aff)
				}
				return
			}
			if aff == nil {
				t.Fatal("want non-nil, got nil")
			}
			check(t, "Initial", aff.Initial, tc.initial)
			check(t, "Final", aff.Final, tc.final)
		})
	}
}
