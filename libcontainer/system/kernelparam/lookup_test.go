package kernelparam

import (
	"testing"
	"testing/fstest"
)

func TestLookupKernelBootParameters(t *testing.T) {
	for _, test := range []struct {
		cmdline                  string
		lookupParameters         []string
		expectedKernelParameters map[string]string
	}{
		{
			cmdline:          "root=/dev/sda1 ro console=ttyS0 console=tty0",
			lookupParameters: []string{"root"},
			expectedKernelParameters: map[string]string{
				"root": "/dev/sda1",
			},
		},
		{
			cmdline:          "ro runc.kernel_parameter=a_value console=ttyS0 console=tty0",
			lookupParameters: []string{"runc.kernel_parameter"},
			expectedKernelParameters: map[string]string{
				"runc.kernel_parameter": "a_value",
			},
		},
		{
			cmdline: "ro runc.kernel_parameter_a=value_a  runc.kernel_parameter_b=value_a:value_b",
			lookupParameters: []string{
				"runc.kernel_parameter_a",
				"runc.kernel_parameter_b",
			},
			expectedKernelParameters: map[string]string{
				"runc.kernel_parameter_a": "value_a",
				"runc.kernel_parameter_b": "value_a:value_b",
			},
		},
		{
			cmdline:                  "root=/dev/sda1 ro console=ttyS0 console=tty0",
			lookupParameters:         []string{"runc.kernel_parameter_a"},
			expectedKernelParameters: map[string]string{},
		},
	} {
		params, err := LookupKernelBootParameters(fstest.MapFS{
			"proc/cmdline": &fstest.MapFile{Data: []byte(test.cmdline + "\n")},
		}, test.lookupParameters...)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if len(params) != len(test.expectedKernelParameters) {
			t.Fatalf("expected %d parameters, got %d", len(test.expectedKernelParameters), len(params))
		}
		for k, v := range test.expectedKernelParameters {
			if params[k] != v {
				t.Fatalf("expected parameter %s to be %s, got %s", k, v, params[k])
			}
		}
	}
}
