// +build linux

package intelrdt

import (
	"io"
	"strings"
	"testing"
)

func TestIntelRdtSetL3CacheSchema(t *testing.T) {
	if !IsCATEnabled() {
		return
	}

	helper := NewIntelRdtTestUtil(t)
	defer helper.cleanup()

	const (
		l3CacheSchemaBefore = "L3:0=f;1=f0"
		l3CacheSchemeAfter  = "L3:0=f0;1=f"
	)

	helper.writeFileContents(map[string]string{
		"schemata": l3CacheSchemaBefore + "\n",
	})

	helper.IntelRdtData.config.IntelRdt.L3CacheSchema = l3CacheSchemeAfter
	intelrdt := NewManager(helper.IntelRdtData.config, "", helper.IntelRdtPath)
	if err := intelrdt.Set(helper.IntelRdtData.config); err != nil {
		t.Fatal(err)
	}

	tmpStrings, err := getIntelRdtParamString(helper.IntelRdtPath, "schemata")
	if err != nil {
		t.Fatalf("Failed to parse file 'schemata' - %s", err)
	}
	values := strings.Split(tmpStrings, "\n")
	value := values[0]

	if value != l3CacheSchemeAfter {
		t.Fatal("Got the wrong value, set 'schemata' failed.")
	}
}

func TestIntelRdtSetMemBwSchema(t *testing.T) {
	if !IsMBAEnabled() {
		return
	}

	helper := NewIntelRdtTestUtil(t)
	defer helper.cleanup()

	const (
		memBwSchemaBefore = "MB:0=20;1=70"
		memBwSchemeAfter  = "MB:0=70;1=20"
	)

	helper.writeFileContents(map[string]string{
		"schemata": memBwSchemaBefore + "\n",
	})

	helper.IntelRdtData.config.IntelRdt.MemBwSchema = memBwSchemeAfter
	intelrdt := NewManager(helper.IntelRdtData.config, "", helper.IntelRdtPath)
	if err := intelrdt.Set(helper.IntelRdtData.config); err != nil {
		t.Fatal(err)
	}

	tmpStrings, err := getIntelRdtParamString(helper.IntelRdtPath, "schemata")
	if err != nil {
		t.Fatalf("Failed to parse file 'schemata' - %s", err)
	}
	values := strings.Split(tmpStrings, "\n")
	value := values[0]

	if value != memBwSchemeAfter {
		t.Fatal("Got the wrong value, set 'schemata' failed.")
	}
}

func TestIntelRdtSetMemBwScSchema(t *testing.T) {
	if !IsMBAScEnabled() {
		return
	}

	helper := NewIntelRdtTestUtil(t)
	defer helper.cleanup()

	const (
		memBwScSchemaBefore = "MB:0=5000;1=7000"
		memBwScSchemeAfter  = "MB:0=9000;1=4000"
	)

	helper.writeFileContents(map[string]string{
		"schemata": memBwScSchemaBefore + "\n",
	})

	helper.IntelRdtData.config.IntelRdt.MemBwSchema = memBwScSchemeAfter
	intelrdt := NewManager(helper.IntelRdtData.config, "", helper.IntelRdtPath)
	if err := intelrdt.Set(helper.IntelRdtData.config); err != nil {
		t.Fatal(err)
	}

	tmpStrings, err := getIntelRdtParamString(helper.IntelRdtPath, "schemata")
	if err != nil {
		t.Fatalf("Failed to parse file 'schemata' - %s", err)
	}
	values := strings.Split(tmpStrings, "\n")
	value := values[0]

	if value != memBwScSchemeAfter {
		t.Fatal("Got the wrong value, set 'schemata' failed.")
	}
}

const (
	mountinfoValid = `18 40 0:18 / /sys rw,nosuid,nodev,noexec,relatime shared:6 - sysfs sysfs rw
￼19 40 0:3 / /proc rw,nosuid,nodev,noexec,relatime shared:5 - proc proc rw
￼20 40 0:5 / /dev rw,nosuid shared:2 - devtmpfs devtmpfs rw,size=131927256k,nr_inodes=32981814,mode=755
￼21 18 0:17 / /sys/kernel/security rw,nosuid,nodev,noexec,relatime shared:7 - securityfs securityfs rw
￼22 20 0:19 / /dev/shm rw,nosuid,nodev shared:3 - tmpfs tmpfs rw
￼23 20 0:12 / /dev/pts rw,nosuid,noexec,relatime shared:4 - devpts devpts rw,gid=5,mode=620,ptmxmode=000
￼24 40 0:20 / /run rw,nosuid,nodev shared:22 - tmpfs tmpfs rw,mode=755
￼25 18 0:21 / /sys/fs/cgroup ro,nosuid,nodev,noexec shared:8 - tmpfs tmpfs ro,mode=755
￼26 25 0:22 / /sys/fs/cgroup/systemd rw,nosuid,nodev,noexec,relatime shared:9 - cgroup cgroup rw,xattr,release_agent=/usr/lib/systemd/systemd-cgroups-agent,name=systemd
￼27 18 0:23 / /sys/fs/pstore rw,nosuid,nodev,noexec,relatime shared:20 - pstore pstore rw
￼28 25 0:24 / /sys/fs/cgroup/perf_event rw,nosuid,nodev,noexec,relatime shared:10 - cgroup cgroup rw,perf_event
￼29 25 0:25 / /sys/fs/cgroup/cpu,cpuacct rw,nosuid,nodev,noexec,relatime shared:11 - cgroup cgroup rw,cpuacct,cpu
￼30 25 0:26 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:12 - cgroup cgroup rw,memory
￼31 25 0:27 / /sys/fs/cgroup/devices rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,devices
￼32 25 0:28 / /sys/fs/cgroup/hugetlb rw,nosuid,nodev,noexec,relatime shared:14 - cgroup cgroup rw,hugetlb
￼33 25 0:29 / /sys/fs/cgroup/blkio rw,nosuid,nodev,noexec,relatime shared:15 - cgroup cgroup rw,blkio
￼34 25 0:30 / /sys/fs/cgroup/pids rw,nosuid,nodev,noexec,relatime shared:16 - cgroup cgroup rw,pids
￼35 25 0:31 / /sys/fs/cgroup/cpuset rw,nosuid,nodev,noexec,relatime shared:17 - cgroup cgroup rw,cpuset
￼36 25 0:32 / /sys/fs/cgroup/freezer rw,nosuid,nodev,noexec,relatime shared:18 - cgroup cgroup rw,freezer
￼37 25 0:33 / /sys/fs/cgroup/net_cls,net_prio rw,nosuid,nodev,noexec,relatime shared:19 - cgroup cgroup rw,net_prio,net_cls
￼38 18 0:34 / /sys/kernel/config rw,relatime shared:21 - configfs configfs rw
￼40 0 253:0 / / rw,relatime shared:1 - ext4 /dev/mapper/vvrg-vvrg rw,data=ordered
￼16 18 0:6 / /sys/kernel/debug rw,relatime shared:23 - debugfs debugfs rw
￼41 18 0:16 / /sys/fs/resctrl rw,relatime shared:24 - resctrl resctrl rw
￼42 20 0:36 / /dev/hugepages rw,relatime shared:25 - hugetlbfs hugetlbfs rw
￼43 19 0:37 / /proc/sys/fs/binfmt_misc rw,relatime shared:26 - autofs systemd-1 rw,fd=32,pgrp=1,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=35492
￼44 20 0:15 / /dev/mqueue rw,relatime shared:27 - mqueue mqueue rw
￼45 40 8:1 / /boot rw,relatime shared:28 - ext4 /dev/sda1 rw,stripe=4,data=ordered
￼46 40 253:1 / /home rw,relatime shared:29 - ext4 /dev/mapper/vvhg-vvhg rw,data=ordered
￼47 40 0:38 / /var/lib/nfs/rpc_pipefs rw,relatime shared:30 - rpc_pipefs sunrpc rw
￼125 24 0:20 /mesos/containers /run/mesos/containers rw,nosuid shared:22 - tmpfs tmpfs rw,mode=755
￼123 40 253:0 /var/lib/docker/containers /var/lib/docker/containers rw,relatime - ext4 /dev/mapper/vvrg-vvrg rw,data=ordered
￼129 40 253:0 /var/lib/docker/overlay2 /var/lib/docker/overlay2 rw,relatime - ext4 /dev/mapper/vvrg-vvrg rw,data=ordered
￼119 24 0:39 / /run/user/1009 rw,nosuid,nodev,relatime shared:100 - tmpfs tmpfs rw,size=26387788k,mode=700,uid=1009,gid=1009`

	mountinfoMbaSc = `18 40 0:18 / /sys rw,nosuid,nodev,noexec,relatime shared:6 - sysfs sysfs rw
19 40 0:3 / /proc rw,nosuid,nodev,noexec,relatime shared:5 - proc proc rw
20 40 0:5 / /dev rw,nosuid shared:2 - devtmpfs devtmpfs rw,size=131927256k,nr_inodes=32981814,mode=755
21 18 0:17 / /sys/kernel/security rw,nosuid,nodev,noexec,relatime shared:7 - securityfs securityfs rw
22 20 0:19 / /dev/shm rw,nosuid,nodev shared:3 - tmpfs tmpfs rw
23 20 0:12 / /dev/pts rw,nosuid,noexec,relatime shared:4 - devpts devpts rw,gid=5,mode=620,ptmxmode=000
24 40 0:20 / /run rw,nosuid,nodev shared:22 - tmpfs tmpfs rw,mode=755
25 18 0:21 / /sys/fs/cgroup ro,nosuid,nodev,noexec shared:8 - tmpfs tmpfs ro,mode=755
26 25 0:22 / /sys/fs/cgroup/systemd rw,nosuid,nodev,noexec,relatime shared:9 - cgroup cgroup rw,xattr,release_agent=/usr/lib/systemd/systemd-cgroups-agent,name=systemd
27 18 0:23 / /sys/fs/pstore rw,nosuid,nodev,noexec,relatime shared:20 - pstore pstore rw
28 25 0:24 / /sys/fs/cgroup/perf_event rw,nosuid,nodev,noexec,relatime shared:10 - cgroup cgroup rw,perf_event
29 25 0:25 / /sys/fs/cgroup/cpu,cpuacct rw,nosuid,nodev,noexec,relatime shared:11 - cgroup cgroup rw,cpuacct,cpu
30 25 0:26 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:12 - cgroup cgroup rw,memory
31 25 0:27 / /sys/fs/cgroup/devices rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,devices
32 25 0:28 / /sys/fs/cgroup/hugetlb rw,nosuid,nodev,noexec,relatime shared:14 - cgroup cgroup rw,hugetlb
33 25 0:29 / /sys/fs/cgroup/blkio rw,nosuid,nodev,noexec,relatime shared:15 - cgroup cgroup rw,blkio
34 25 0:30 / /sys/fs/cgroup/pids rw,nosuid,nodev,noexec,relatime shared:16 - cgroup cgroup rw,pids
35 25 0:31 / /sys/fs/cgroup/cpuset rw,nosuid,nodev,noexec,relatime shared:17 - cgroup cgroup rw,cpuset
36 25 0:32 / /sys/fs/cgroup/freezer rw,nosuid,nodev,noexec,relatime shared:18 - cgroup cgroup rw,freezer
37 25 0:33 / /sys/fs/cgroup/net_cls,net_prio rw,nosuid,nodev,noexec,relatime shared:19 - cgroup cgroup rw,net_prio,net_cls
38 18 0:34 / /sys/kernel/config rw,relatime shared:21 - configfs configfs rw
40 0 253:0 / / rw,relatime shared:1 - ext4 /dev/mapper/vvrg-vvrg rw,data=ordered
16 18 0:6 / /sys/kernel/debug rw,relatime shared:23 - debugfs debugfs rw
41 18 0:16 / /sys/fs/resctrl rw,relatime shared:24 - resctrl resctrl rw,mba_MBps
42 20 0:36 / /dev/hugepages rw,relatime shared:25 - hugetlbfs hugetlbfs rw
43 19 0:37 / /proc/sys/fs/binfmt_misc rw,relatime shared:26 - autofs systemd-1 rw,fd=32,pgrp=1,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=35492
44 20 0:15 / /dev/mqueue rw,relatime shared:27 - mqueue mqueue rw
45 40 8:1 / /boot rw,relatime shared:28 - ext4 /dev/sda1 rw,stripe=4,data=ordered
46 40 253:1 / /home rw,relatime shared:29 - ext4 /dev/mapper/vvhg-vvhg rw,data=ordered
47 40 0:38 / /var/lib/nfs/rpc_pipefs rw,relatime shared:30 - rpc_pipefs sunrpc rw
125 24 0:20 /mesos/containers /run/mesos/containers rw,nosuid shared:22 - tmpfs tmpfs rw,mode=755
123 40 253:0 /var/lib/docker/containers /var/lib/docker/containers rw,relatime - ext4 /dev/mapper/vvrg-vvrg rw,data=ordered
129 40 253:0 /var/lib/docker/overlay2 /var/lib/docker/overlay2 rw,relatime - ext4 /dev/mapper/vvrg-vvrg rw,data=ordered
119 24 0:39 / /run/user/1009 rw,nosuid,nodev,relatime shared:100 - tmpfs tmpfs rw,size=26387788k,mode=700,uid=1009,gid=1009`
)

func TestFindIntelRdtMountpointDir(t *testing.T) {
	testCases := []struct {
		name            string
		input           io.Reader
		isNotFoundError bool
		isError         bool
		mbaScEnabled    bool
		mountpoint      string
	}{
		{
			name:       "Valid mountinfo with MBA Software Controller disabled",
			input:      strings.NewReader(mountinfoValid),
			mountpoint: "/sys/fs/resctrl",
		},
		{
			name:         "Valid mountinfo with MBA Software Controller enabled",
			input:        strings.NewReader(mountinfoMbaSc),
			mbaScEnabled: true,
			mountpoint:   "/sys/fs/resctrl",
		},
		{
			name:            "Empty mountinfo",
			input:           strings.NewReader(""),
			isNotFoundError: true,
		},
		{
			name:    "Broken mountinfo",
			input:   strings.NewReader("baa"),
			isError: true,
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mp, err := findIntelRdtMountpointDir(tc.input)
			if tc.isNotFoundError {
				if !IsNotFound(err) {
					t.Errorf("expected IsNotFound error, got %+v", err)
				}
				return
			}
			if tc.isError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("expected nil, got %+v", err)
				return
			}
			// no errors, check the results
			if tc.mbaScEnabled != mbaScEnabled {
				t.Errorf("expected mbaScEnabled=%v, got %v",
					tc.mbaScEnabled, mbaScEnabled)
			}
			if tc.mountpoint != mp {
				t.Errorf("expected mountpoint=%q, got %q",
					tc.mountpoint, mp)
			}
		})
	}
}
