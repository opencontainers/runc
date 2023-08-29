package libcontainer

import (
	"os/user"
	"slices"
	"strconv"
	"testing"
)

func TestPrepareEnv(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	home := "HOME=" + u.HomeDir
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		env, wantEnv []string
	}{
		{
			env:     []string{},
			wantEnv: []string{home},
		},
		{
			env:     []string{"HOME=/whoo", "FOO=bar"},
			wantEnv: []string{"HOME=/whoo", "FOO=bar"},
		},
		{
			env:     []string{"A=a", "A=b", "A=c"},
			wantEnv: []string{"A=c", home},
		},
		{
			env:     []string{"TERM=vt100", "HOME=/home/one", "HOME=/home/two", "TERM=xterm", "HOME=/home/three", "FOO=bar"},
			wantEnv: []string{"TERM=xterm", "HOME=/home/three", "FOO=bar"},
		},
	}

	for _, tc := range tests {
		env, err := prepareEnv(tc.env, uid)
		if err != nil {
			t.Error(err)
			continue
		}
		if !slices.Equal(env, tc.wantEnv) {
			t.Errorf("want %v, got %v", tc.wantEnv, env)
		}
	}
}
