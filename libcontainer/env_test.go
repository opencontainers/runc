package libcontainer

import (
	"slices"
	"testing"
)

func TestPrepareEnvDedup(t *testing.T) {
	tests := []struct {
		env, wantEnv []string
	}{
		{
			env:     []string{},
			wantEnv: []string{},
		},
		{
			env:     []string{"HOME=/root", "FOO=bar"},
			wantEnv: []string{"HOME=/root", "FOO=bar"},
		},
		{
			env:     []string{"A=a", "A=b", "A=c"},
			wantEnv: []string{"A=c"},
		},
		{
			env:     []string{"TERM=vt100", "HOME=/home/one", "HOME=/home/two", "TERM=xterm", "HOME=/home/three", "FOO=bar"},
			wantEnv: []string{"TERM=xterm", "HOME=/home/three", "FOO=bar"},
		},
	}

	for _, tc := range tests {
		env, _, err := prepareEnv(tc.env)
		if err != nil {
			t.Error(err)
			continue
		}
		if !slices.Equal(env, tc.wantEnv) {
			t.Errorf("want %v, got %v", tc.wantEnv, env)
		}
	}
}
