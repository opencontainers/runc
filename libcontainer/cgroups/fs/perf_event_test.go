package fs

import (
	"testing"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestPerfEventSet(t *testing.T) {
	helper := NewCgroupTestUtil("perf_event", t)

	firstEvent := configs.PerfEvent{Config: 30, Ext1: 21, Ext2: 10}
	firstEventFd := firstEvent.Config
	secondEvent := configs.PerfEvent{Config: 22}
	secondEventFd := secondEvent.Config

	helper.CgroupData.config.Resources.PerfEvents = [][]configs.PerfEvent{
		{
			firstEvent, secondEvent,
		},
		{
			secondEvent, firstEvent,
		},
	}

	perfEvent := &PerfEventGroup{
		PerfEventOpen: func(attr *unix.PerfEventAttr, pid int, cpu int, groupFd int, flags int) (fd int, err error) {
			return int(attr.Config), nil
		},
	}

	if err := perfEvent.Set(helper.CgroupPath, helper.CgroupData.config.Resources); err != nil {
		t.Fatal(err)
	}

	if len(perfEvent.events) != 2 {
		t.Fatal("there should be two perf events group")
	}

	// First group.
	if firstEvent.Config != perfEvent.events[0][0].event.Config ||
		firstEvent.Ext1 != perfEvent.events[0][0].event.Ext1 ||
		firstEvent.Ext2 != perfEvent.events[0][0].event.Ext2 {
		t.Fatal("First element of first group should be first event.")
	}

	if firstEventFd != perfEvent.events[0][0].fd {
		t.Fatalf("First element fd of first group should be equal %v but got %v.", firstEventFd, perfEvent.events[0][0].fd)
	}

	if secondEvent.Config != perfEvent.events[0][1].event.Config ||
		secondEvent.Ext1 != perfEvent.events[0][1].event.Ext1 ||
		secondEvent.Ext2 != perfEvent.events[0][1].event.Ext2 {
		t.Fatal("Second element of first group should be second event.")
	}

	if secondEventFd != perfEvent.events[0][1].fd {
		t.Fatalf("Second element fd of first group should be equal %v but got %v.", secondEventFd, perfEvent.events[0][1].fd)
	}

	// Second group.
	if secondEvent.Config != perfEvent.events[1][0].event.Config ||
		secondEvent.Ext1 != perfEvent.events[1][0].event.Ext1 ||
		secondEvent.Ext2 != perfEvent.events[1][0].event.Ext2 {
		t.Fatal("First element of second group should be second event.")
	}

	if secondEventFd != perfEvent.events[1][0].fd {
		t.Fatalf("First element fd of second group should be equal %v but got %v.", secondEventFd, perfEvent.events[1][0].fd)
	}

	if firstEvent.Config != perfEvent.events[1][1].event.Config ||
		firstEvent.Ext1 != perfEvent.events[1][1].event.Ext1 ||
		firstEvent.Ext2 != perfEvent.events[1][1].event.Ext2 {
		t.Fatal("Second element of second group should be first event.")
	}

	if firstEventFd != perfEvent.events[1][1].fd {
		t.Fatalf("Second element fd of second group should be equal %v but got %v.", firstEventFd, perfEvent.events[1][1].fd)
	}

}
