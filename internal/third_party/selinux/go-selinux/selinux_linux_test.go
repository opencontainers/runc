package selinux

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSetFileLabel(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	const (
		tmpFile = "selinux_test"
		tmpLink = "selinux_test_link"
		con     = "system_u:object_r:bin_t:s0:c1,c2"
		con2    = "system_u:object_r:bin_t:s0:c3,c4"
	)

	_ = os.Remove(tmpFile)
	out, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE, 0)
	if err != nil {
		t.Fatal(err)
	}
	out.Close()
	defer os.Remove(tmpFile)

	_ = os.Remove(tmpLink)
	if err := os.Symlink(tmpFile, tmpLink); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpLink)

	if err := SetFileLabel(tmpLink, con); err != nil {
		t.Fatalf("SetFileLabel failed: %s", err)
	}
	filelabel, err := FileLabel(tmpLink)
	if err != nil {
		t.Fatalf("FileLabel failed: %s", err)
	}
	if filelabel != con {
		t.Fatalf("FileLabel failed, returned %s expected %s", filelabel, con)
	}

	// Using LfileLabel to verify that the symlink itself is not labeled.
	linkLabel, err := LfileLabel(tmpLink)
	if err != nil {
		t.Fatalf("LfileLabel failed: %s", err)
	}
	if linkLabel == con {
		t.Fatalf("Label on symlink should not be set, got: %q", linkLabel)
	}

	// Use LsetFileLabel to set a label on the symlink itself.
	if err := LsetFileLabel(tmpLink, con2); err != nil {
		t.Fatalf("LsetFileLabel failed: %s", err)
	}
	filelabel, err = FileLabel(tmpFile)
	if err != nil {
		t.Fatalf("FileLabel failed: %s", err)
	}
	if filelabel != con {
		t.Fatalf("FileLabel was updated, returned %s expected %s", filelabel, con)
	}

	linkLabel, err = LfileLabel(tmpLink)
	if err != nil {
		t.Fatalf("LfileLabel failed: %s", err)
	}
	if linkLabel != con2 {
		t.Fatalf("LfileLabel failed: returned %s expected %s", linkLabel, con2)
	}
}

func TestKVMLabels(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	plabel, flabel := KVMContainerLabels()
	if plabel == "" {
		t.Log("Failed to read kvm label")
	}
	t.Log(plabel)
	t.Log(flabel)
	if _, err := CanonicalizeContext(plabel); err != nil {
		t.Fatal(err)
	}
	if _, err := CanonicalizeContext(flabel); err != nil {
		t.Fatal(err)
	}

	ReleaseLabel(plabel)
}

func TestInitLabels(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	plabel, flabel := InitContainerLabels()
	if plabel == "" {
		t.Log("Failed to read init label")
	}
	t.Log(plabel)
	t.Log(flabel)
	if _, err := CanonicalizeContext(plabel); err != nil {
		t.Fatal(err)
	}
	if _, err := CanonicalizeContext(flabel); err != nil {
		t.Fatal(err)
	}
	ReleaseLabel(plabel)
}

func TestDuplicateLabel(t *testing.T) {
	secopt, err := DupSecOpt("system_u:system_r:container_t:s0:c1,c2")
	if err != nil {
		t.Fatalf("DupSecOpt: %v", err)
	}
	for _, opt := range secopt {
		con := strings.SplitN(opt, ":", 2)
		if con[0] == "user" {
			if con[1] != "system_u" {
				t.Errorf("DupSecOpt Failed user incorrect")
			}
			continue
		}
		if con[0] == "role" {
			if con[1] != "system_r" {
				t.Errorf("DupSecOpt Failed role incorrect")
			}
			continue
		}
		if con[0] == "type" {
			if con[1] != "container_t" {
				t.Errorf("DupSecOpt Failed type incorrect")
			}
			continue
		}
		if con[0] == "level" {
			if con[1] != "s0:c1,c2" {
				t.Errorf("DupSecOpt Failed level incorrect")
			}
			continue
		}
		t.Errorf("DupSecOpt failed: invalid field %q", con[0])
	}
	secopt = DisableSecOpt()
	if secopt[0] != "disable" {
		t.Errorf(`DisableSecOpt failed: want "disable", got %q`, secopt[0])
	}
}

func TestSELinuxNoLevel(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	tlabel := "system_u:system_r:container_t"
	dup, err := DupSecOpt(tlabel)
	if err != nil {
		t.Fatal(err)
	}

	if len(dup) != 3 {
		t.Errorf("DupSecOpt failed on non mls label: want 3, got %d", len(dup))
	}
	con, err := NewContext(tlabel)
	if err != nil {
		t.Fatal(err)
	}
	if con.Get() != tlabel {
		t.Errorf("NewContext and con.Get() failed on non mls label: want %q, got %q", tlabel, con.Get())
	}
}

func TestSocketLabel(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	// Ensure the thread stays the same for duration of the test.
	// Otherwise Go runtime can switch this to a different thread,
	// which results in EACCES in call to SetSocketLabel.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	label := "system_u:object_r:container_t:s0:c1,c2"
	if err := SetSocketLabel(label); err != nil {
		t.Fatal(err)
	}
	nlabel, err := SocketLabel()
	if err != nil {
		t.Fatal(err)
	}
	if label != nlabel {
		t.Errorf("SocketLabel %s != %s", nlabel, label)
	}
}

func TestKeyLabel(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	// Ensure the thread stays the same for duration of the test.
	// Otherwise Go runtime can switch this to a different thread,
	// which results in EACCES in call to SetKeyLabel.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if unix.Getpid() != unix.Gettid() {
		t.Skip(ErrNotTGLeader)
	}

	label := "system_u:object_r:container_t:s0:c1,c2"
	if err := SetKeyLabel(label); err != nil {
		t.Fatal(err)
	}
	nlabel, err := KeyLabel()
	if err != nil {
		t.Fatal(err)
	}
	if label != nlabel {
		t.Errorf("KeyLabel: want %q, got %q", label, nlabel)
	}
}

func BenchmarkContextGet(b *testing.B) {
	ctx, err := NewContext("system_u:object_r:container_file_t:s0:c1022,c1023")
	if err != nil {
		b.Fatal(err)
	}
	str := ""
	for i := 0; i < b.N; i++ {
		str = ctx.get()
	}
	b.Log(str)
}

func TestSELinux(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	// Ensure the thread stays the same for duration of the test.
	// Otherwise Go runtime can switch this to a different thread,
	// which results in EACCES in call to SetFSCreateLabel.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var (
		err            error
		plabel, flabel string
	)

	plabel, flabel = ContainerLabels()
	t.Log(plabel)
	t.Log(flabel)
	plabel, flabel = ContainerLabels()
	t.Log(plabel)
	t.Log(flabel)
	ReleaseLabel(plabel)

	plabel, flabel = ContainerLabels()
	t.Log(plabel)
	t.Log(flabel)
	ClearLabels()
	t.Log("ClearLabels")
	plabel, flabel = ContainerLabels()
	t.Log(plabel)
	t.Log(flabel)
	ReleaseLabel(plabel)

	pid := os.Getpid()
	t.Logf("PID:%d MCS:%s", pid, intToMcs(pid, 1023))
	err = SetFSCreateLabel("unconfined_u:unconfined_r:unconfined_t:s0")
	if err != nil {
		t.Fatal("SetFSCreateLabel failed:", err)
	}
	t.Log(FSCreateLabel())
	err = SetFSCreateLabel("")
	if err != nil {
		t.Fatal("SetFSCreateLabel failed:", err)
	}
	t.Log(FSCreateLabel())
	t.Log(PidLabel(1))
}

func TestSetEnforceMode(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}
	if os.Geteuid() != 0 {
		t.Skip("root required, skipping")
	}

	t.Log("Enforcing Mode:", EnforceMode())
	mode := DefaultEnforceMode()
	t.Log("Default Enforce Mode:", mode)
	defer func() {
		_ = SetEnforceMode(mode)
	}()

	if err := SetEnforceMode(Enforcing); err != nil {
		t.Fatalf("setting selinux mode to enforcing failed: %v", err)
	}
	if err := SetEnforceMode(Permissive); err != nil {
		t.Fatalf("setting selinux mode to permissive failed: %v", err)
	}
}

func TestCanonicalizeContext(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	con := "system_u:object_r:bin_t:s0:c1,c2,c3"
	checkcon := "system_u:object_r:bin_t:s0:c1.c3"
	newcon, err := CanonicalizeContext(con)
	if err != nil {
		t.Fatal(err)
	}
	if newcon != checkcon {
		t.Fatalf("CanonicalizeContext(%s) returned %s expected %s", con, newcon, checkcon)
	}
	con = "system_u:object_r:bin_t:s0:c5,c2"
	checkcon = "system_u:object_r:bin_t:s0:c2,c5"
	newcon, err = CanonicalizeContext(con)
	if err != nil {
		t.Fatal(err)
	}
	if newcon != checkcon {
		t.Fatalf("CanonicalizeContext(%s) returned %s expected %s", con, newcon, checkcon)
	}
}

func TestFindSELinuxfsInMountinfo(t *testing.T) {
	//nolint:dupword // ignore duplicate words (sysfs sysfs)
	const mountinfo = `18 62 0:17 / /sys rw,nosuid,nodev,noexec,relatime shared:6 - sysfs sysfs rw,seclabel
19 62 0:3 / /proc rw,nosuid,nodev,noexec,relatime shared:5 - proc proc rw
20 62 0:5 / /dev rw,nosuid shared:2 - devtmpfs devtmpfs rw,seclabel,size=3995472k,nr_inodes=998868,mode=755
21 18 0:16 / /sys/kernel/security rw,nosuid,nodev,noexec,relatime shared:7 - securityfs securityfs rw
22 20 0:18 / /dev/shm rw,nosuid,nodev shared:3 - tmpfs tmpfs rw,seclabel
23 20 0:11 / /dev/pts rw,nosuid,noexec,relatime shared:4 - devpts devpts rw,seclabel,gid=5,mode=620,ptmxmode=000
24 62 0:19 / /run rw,nosuid,nodev shared:23 - tmpfs tmpfs rw,seclabel,mode=755
25 18 0:20 / /sys/fs/cgroup ro,nosuid,nodev,noexec shared:8 - tmpfs tmpfs ro,seclabel,mode=755
26 25 0:21 / /sys/fs/cgroup/systemd rw,nosuid,nodev,noexec,relatime shared:9 - cgroup cgroup rw,xattr,release_agent=/usr/lib/systemd/systemd-cgroups-agent,name=systemd
27 18 0:22 / /sys/fs/pstore rw,nosuid,nodev,noexec,relatime shared:20 - pstore pstore rw
28 25 0:23 / /sys/fs/cgroup/perf_event rw,nosuid,nodev,noexec,relatime shared:10 - cgroup cgroup rw,perf_event
29 25 0:24 / /sys/fs/cgroup/devices rw,nosuid,nodev,noexec,relatime shared:11 - cgroup cgroup rw,devices
30 25 0:25 / /sys/fs/cgroup/cpu,cpuacct rw,nosuid,nodev,noexec,relatime shared:12 - cgroup cgroup rw,cpuacct,cpu
31 25 0:26 / /sys/fs/cgroup/freezer rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,freezer
32 25 0:27 / /sys/fs/cgroup/net_cls,net_prio rw,nosuid,nodev,noexec,relatime shared:14 - cgroup cgroup rw,net_prio,net_cls
33 25 0:28 / /sys/fs/cgroup/cpuset rw,nosuid,nodev,noexec,relatime shared:15 - cgroup cgroup rw,cpuset
34 25 0:29 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:16 - cgroup cgroup rw,memory
35 25 0:30 / /sys/fs/cgroup/pids rw,nosuid,nodev,noexec,relatime shared:17 - cgroup cgroup rw,pids
36 25 0:31 / /sys/fs/cgroup/hugetlb rw,nosuid,nodev,noexec,relatime shared:18 - cgroup cgroup rw,hugetlb
37 25 0:32 / /sys/fs/cgroup/blkio rw,nosuid,nodev,noexec,relatime shared:19 - cgroup cgroup rw,blkio
59 18 0:33 / /sys/kernel/config rw,relatime shared:21 - configfs configfs rw
62 1 253:1 / / rw,relatime shared:1 - ext4 /dev/vda1 rw,seclabel,data=ordered
38 18 0:15 / /sys/fs/selinux rw,relatime shared:22 - selinuxfs selinuxfs rw
39 19 0:35 / /proc/sys/fs/binfmt_misc rw,relatime shared:24 - autofs systemd-1 rw,fd=29,pgrp=1,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=11601
40 20 0:36 / /dev/hugepages rw,relatime shared:25 - hugetlbfs hugetlbfs rw,seclabel
41 20 0:14 / /dev/mqueue rw,relatime shared:26 - mqueue mqueue rw,seclabel
42 18 0:6 / /sys/kernel/debug rw,relatime shared:27 - debugfs debugfs rw
112 62 253:1 /var/lib/docker/plugins /var/lib/docker/plugins rw,relatime - ext4 /dev/vda1 rw,seclabel,data=ordered
115 62 253:1 /var/lib/docker/overlay2 /var/lib/docker/overlay2 rw,relatime - ext4 /dev/vda1 rw,seclabel,data=ordered
118 62 7:0 / /root/mnt rw,relatime shared:66 - ext4 /dev/loop0 rw,seclabel,data=ordered
121 115 0:38 / /var/lib/docker/overlay2/8cdbabf81bc89b14ea54eaf418c1922068f06917fff57e184aa26541ff291073/merged rw,relatime - overlay overlay rw,seclabel,lowerdir=/var/lib/docker/overlay2/l/CPD4XI7UD4GGTGSJVPQSHWZKTK:/var/lib/docker/overlay2/l/NQKORR3IS7KNQDER35AZECLH4Z,upperdir=/var/lib/docker/overlay2/8cdbabf81bc89b14ea54eaf418c1922068f06917fff57e184aa26541ff291073/diff,workdir=/var/lib/docker/overlay2/8cdbabf81bc89b14ea54eaf418c1922068f06917fff57e184aa26541ff291073/work
125 62 0:39 / /var/lib/docker/containers/5e3fce422957c291a5b502c2cf33d512fc1fcac424e4113136c808360e5b7215/shm rw,nosuid,nodev,noexec,relatime shared:68 - tmpfs shm rw,seclabel,size=65536k
186 24 0:3 / /run/docker/netns/0a08e7496c6d rw,nosuid,nodev,noexec,relatime shared:5 - proc proc rw
130 62 0:15 / /root/chroot/selinux rw,relatime shared:22 - selinuxfs selinuxfs rw
109 24 0:37 / /run/user/0 rw,nosuid,nodev,relatime shared:62 - tmpfs tmpfs rw,seclabel,size=801032k,mode=700
`
	s := bufio.NewScanner(bytes.NewBuffer([]byte(mountinfo)))
	for _, expected := range []string{"/sys/fs/selinux", "/root/chroot/selinux", ""} {
		mnt := findSELinuxfsMount(s)
		t.Logf("found %q", mnt)
		if mnt != expected {
			t.Fatalf("expected %q, got %q", expected, mnt)
		}
	}
}

func TestSecurityCheckContext(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	// check with valid context
	context, err := CurrentLabel()
	if err != nil {
		t.Fatalf("CurrentLabel() error: %v", err)
	}
	if context != "" {
		t.Logf("SecurityCheckContext(%q)", context)
		err = SecurityCheckContext(context)
		if err != nil {
			t.Errorf("SecurityCheckContext(%q) error: %v", context, err)
		}
	}

	context = "not-syntactically-valid"
	err = SecurityCheckContext(context)
	if err == nil {
		t.Errorf("SecurityCheckContext(%q) succeeded, expected to fail", context)
	}
}

func TestClassIndex(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	idx, err := ClassIndex("process")
	if err != nil {
		t.Errorf("Classindex error: %v", err)
	}
	// Every known policy has process as index 2, but it isn't guaranteed
	if idx != 2 {
		t.Errorf("ClassIndex unexpected answer %d, possibly not reference policy", idx)
	}

	_, err = ClassIndex("foobar")
	if err == nil {
		t.Errorf("ClassIndex(\"foobar\") succeeded, expected to fail:")
	}
}

func TestComputeCreateContext(t *testing.T) {
	if !GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}

	// This may or may not be in the loaded policy but any refpolicy based policy should have it
	init := "system_u:system_r:init_t:s0"
	tmp := "system_u:object_r:tmp_t:s0"
	file := "file"
	t.Logf("ComputeCreateContext(%s, %s, %s)", init, tmp, file)
	context, err := ComputeCreateContext(init, tmp, file)
	if err != nil {
		t.Errorf("ComputeCreateContext error: %v", err)
	}
	if context != "system_u:object_r:init_tmp_t:s0" {
		t.Errorf("ComputeCreateContext unexpected answer %s, possibly not reference policy", context)
	}

	badcon := "badcon"
	process := "process"
	// Test to ensure that a bad context returns an error
	t.Logf("ComputeCreateContext(%s, %s, %s)", badcon, tmp, process)
	_, err = ComputeCreateContext(badcon, tmp, process)
	if err == nil {
		t.Errorf("ComputeCreateContext(%s, %s, %s) succeeded, expected failure", badcon, tmp, process)
	}
}

func TestGlbLub(t *testing.T) {
	tests := []struct {
		expectedErr   error
		sourceRange   string
		targetRange   string
		expectedRange string
	}{
		{
			sourceRange:   "s0:c0.c100-s10:c0.c150",
			targetRange:   "s5:c50.c100-s15:c0.c149",
			expectedRange: "s5:c50.c100-s10:c0.c149",
		},
		{
			sourceRange:   "s5:c50.c100-s15:c0.c149",
			targetRange:   "s0:c0.c100-s10:c0.c150",
			expectedRange: "s5:c50.c100-s10:c0.c149",
		},
		{
			sourceRange:   "s0:c0.c100-s10:c0.c150",
			targetRange:   "s0",
			expectedRange: "s0",
		},
		{
			sourceRange:   "s6:c0.c1023",
			targetRange:   "s6:c0,c2,c11,c201.c429,c431.c511",
			expectedRange: "s6:c0,c2,c11,c201.c429,c431.c511",
		},
		{
			sourceRange:   "s0-s15:c0.c1023",
			targetRange:   "s6:c0,c2,c11,c201.c429,c431.c511",
			expectedRange: "s6-s6:c0,c2,c11,c201.c429,c431.c511",
		},
		{
			sourceRange:   "s0:c0.c100,c125,c140,c150-s10",
			targetRange:   "s4:c0.c50,c140",
			expectedRange: "s4:c0.c50,c140-s4",
		},
		{
			sourceRange:   "s5:c512.c550,c552.c1023-s5:c0.c550,c552.c1023",
			targetRange:   "s5:c512.c550,c553.c1023-s5:c0,c1,c4,c5,c6,c512.c550,c553.c1023",
			expectedRange: "s5:c512.c550,c553.c1023-s5:c0,c1,c4.c6,c512.c550,c553.c1023",
		},
		{
			sourceRange:   "s5:c512.c540,c542,c543,c552.c1023-s5:c0.c550,c552.c1023",
			targetRange:   "s5:c512.c550,c553.c1023-s5:c0,c1,c4,c5,c6,c512.c550,c553.c1023",
			expectedRange: "s5:c512.c540,c542,c543,c553.c1023-s5:c0,c1,c4.c6,c512.c550,c553.c1023",
		},
		{
			sourceRange:   "s5:c50.c100-s15:c0.c149",
			targetRange:   "s5:c512.c550,c552.c1023-s5:c0.c550,c552.c1023",
			expectedRange: "s5-s5:c0.c149",
		},
		{
			sourceRange:   "s5-s15",
			targetRange:   "s6-s7",
			expectedRange: "s6-s7",
		},
		{
			sourceRange: "s5:c50.c100-s15:c0.c149",
			targetRange: "s4-s4:c0.c1023",
			expectedErr: ErrIncomparable,
		},
		{
			sourceRange: "s4-s4:c0.c1023",
			targetRange: "s5:c50.c100-s15:c0.c149",
			expectedErr: ErrIncomparable,
		},
		{
			sourceRange: "s4-s4:c0.c1023.c10000",
			targetRange: "s5:c50.c100-s15:c0.c149",
			expectedErr: strconv.ErrSyntax,
		},
		{
			sourceRange: "s4-s4:c0.c1023.c10000-s4",
			targetRange: "s5:c50.c100-s15:c0.c149-s5",
			expectedErr: strconv.ErrSyntax,
		},
		{
			sourceRange: "4-4",
			targetRange: "s5:c50.c100-s15:c0.c149",
			expectedErr: ErrLevelSyntax,
		},
		{
			sourceRange: "t4-t4",
			targetRange: "s5:c50.c100-s15:c0.c149",
			expectedErr: ErrLevelSyntax,
		},
		{
			sourceRange: "s5:x50.x100-s15:c0.c149",
			targetRange: "s5:c50.c100-s15:c0.c149",
			expectedErr: ErrLevelSyntax,
		},
	}

	for _, tt := range tests {
		got, err := CalculateGlbLub(tt.sourceRange, tt.targetRange)
		if !errors.Is(err, tt.expectedErr) {
			// Go 1.13 strconv errors are not unwrappable,
			// so do that manually.
			// TODO remove this once we stop supporting Go 1.13.
			var numErr *strconv.NumError
			if errors.As(err, &numErr) && numErr.Err == tt.expectedErr { //nolint:errorlint // see above
				continue
			}
			t.Fatalf("want %q got %q: src: %q tgt: %q", tt.expectedErr, err, tt.sourceRange, tt.targetRange)
		}

		if got != tt.expectedRange {
			t.Errorf("want %q got %q", tt.expectedRange, got)
		}
	}
}

func TestContextWithLevel(t *testing.T) {
	want := "bob:sysadm_r:sysadm_t:SystemLow-SystemHigh"

	goodDefaultBuff := `
foo_r:foo_t:s0     sysadm_r:sysadm_t:s0
staff_r:staff_t:s0                 baz_r:baz_t:s0   sysadm_r:sysadm_t:s0
`

	verifier := func(con string) error {
		if con != want {
			return fmt.Errorf("invalid context %s", con)
		}

		return nil
	}

	tests := []struct {
		name, userBuff, defaultBuff string
	}{
		{
			name: "match exists in user context file",
			userBuff: `# COMMENT
foo_r:foo_t:s0     sysadm_r:sysadm_t:s0

staff_r:staff_t:s0                 baz_r:baz_t:s0   sysadm_r:sysadm_t:s0
`,
			defaultBuff: goodDefaultBuff,
		},
		{
			name: "match exists in default context file, but not in user file",
			userBuff: `# COMMENT
foo_r:foo_t:s0     sysadm_r:sysadm_t:s0
fake_r:fake_t:s0                 baz_r:baz_t:s0   sysadm_r:sysadm_t:s0
`,
			defaultBuff: goodDefaultBuff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := defaultSECtx{
				user:       "bob",
				level:      "SystemLow-SystemHigh",
				scon:       "system_u:staff_r:staff_t:s0",
				userRdr:    bytes.NewBufferString(tt.userBuff),
				defaultRdr: bytes.NewBufferString(tt.defaultBuff),
				verifier:   verifier,
			}

			got, err := getDefaultContextFromReaders(&c)
			if err != nil {
				t.Fatalf("err should not exist but is: %v", err)
			}

			if got != want {
				t.Fatalf("got context: %q but expected %q", got, want)
			}
		})
	}

	t.Run("no match in user or default context files", func(t *testing.T) {
		badUserBuff := ""

		badDefaultBuff := `
		foo_r:foo_t:s0     sysadm_r:sysadm_t:s0
		dne_r:dne_t:s0                 baz_r:baz_t:s0   sysadm_r:sysadm_t:s0
		`
		c := defaultSECtx{
			user:       "bob",
			level:      "SystemLow-SystemHigh",
			scon:       "system_u:staff_r:staff_t:s0",
			userRdr:    bytes.NewBufferString(badUserBuff),
			defaultRdr: bytes.NewBufferString(badDefaultBuff),
			verifier:   verifier,
		}

		_, err := getDefaultContextFromReaders(&c)
		if err == nil {
			t.Fatalf("err was expected")
		}
	})
}

func BenchmarkChcon(b *testing.B) {
	file, err := filepath.Abs(os.Args[0])
	if err != nil {
		b.Fatalf("filepath.Abs: %v", err)
	}
	dir := filepath.Dir(file)
	con, err := FileLabel(file)
	if err != nil {
		b.Fatalf("FileLabel(%q): %v", file, err)
	}
	b.Logf("Chcon(%q, %q)", dir, con)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if err := Chcon(dir, con, true); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCurrentLabel(b *testing.B) {
	var (
		l   string
		err error
	)
	for n := 0; n < b.N; n++ {
		l, err = CurrentLabel()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.Log(l)
}

func BenchmarkReadConfig(b *testing.B) {
	str := ""
	for n := 0; n < b.N; n++ {
		str = readConfig(selinuxTypeTag)
	}
	b.Log(str)
}

func BenchmarkLoadLabels(b *testing.B) {
	for n := 0; n < b.N; n++ {
		loadLabels()
	}
}
