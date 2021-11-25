package fs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"unsafe"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	anyCPU    = -1 // Parameter which indicates measuring on any CPU.
	leaderFD  = -1
	perfFlags = unix.PERF_FLAG_FD_CLOEXEC | unix.PERF_FLAG_PID_CGROUP
)

type PerfEvent struct {
	event configs.PerfEvent
	fd    int
}

func (e *PerfEvent) String() string {
	return fmt.Sprintf("%d%d%d", e.event.Config, e.event.Ext1, e.event.Ext2)
}

type PerfEventGroup struct {
	PerfEventOpen func(attr *unix.PerfEventAttr, pid int, cpu int, groupFd int, flags int) (fd int, err error)
	//ioctlSetInt   func(fd int, req uint, value int) error
	events [][]PerfEvent
}

func (s *PerfEventGroup) Name() string {
	return "perf_event"
}

func (s *PerfEventGroup) Apply(path string, _ *configs.Resources, pid int) error {
	return apply(path, pid)
}

func (s *PerfEventGroup) preparePerfEventAttr(event configs.PerfEvent, isLeader bool) *unix.PerfEventAttr {
	attrs := &unix.PerfEventAttr{
		Sample_type: unix.PERF_SAMPLE_IDENTIFIER,
		Read_format: unix.PERF_FORMAT_TOTAL_TIME_ENABLED | unix.PERF_FORMAT_TOTAL_TIME_RUNNING |
			unix.PERF_FORMAT_GROUP | unix.PERF_FORMAT_ID,
		Bits:   unix.PerfBitInherit,
		Config: uint64(event.Config),
		Ext1:   uint64(event.Ext1),
		Ext2:   uint64(event.Ext2),
		Size:   uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
	}

	if isLeader {
		attrs.Bits |= unix.PerfBitDisabled
	}

	return attrs
}

func (s *PerfEventGroup) Set(path string, r *configs.Resources) error {
	if len(r.PerfEvents) == 0 {
		return nil
	}
	s.events = make([][]PerfEvent, len(r.PerfEvents))

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	cgroupFD := int(file.Fd())

	for index, group := range r.PerfEvents {
		if len(group) == 0 {
			continue
		}

		// Prepare the leader.
		attrs := s.preparePerfEventAttr(group[0], true)
		leaderFD, err := s.PerfEventOpen(attrs, cgroupFD, 0, leaderFD, perfFlags)
		if err != nil {
			return fmt.Errorf("couldn't register perf event %+v, %w", attrs, err)
		}

		s.events[index] = make([]PerfEvent, len(group))
		s.events[index][0] = PerfEvent{
			event: group[0],
			fd:    leaderFD,
		}

		// Prepare rest of the group.
		for i, event := range group[1:] {
			attrs := s.preparePerfEventAttr(event, false)
			fd, err := s.PerfEventOpen(attrs, cgroupFD, anyCPU, leaderFD, perfFlags)
			if err != nil {
				return err
			}
			s.events[index][i+1] = PerfEvent{
				event: event,
				fd:    fd,
			}
		}
	}

	return nil
}

// GroupReadFormat allows to read perf event's values for grouped events.
// See https://man7.org/linux/man-pages/man2/perf_event_open.2.html section "Reading results" with PERF_FORMAT_GROUP specified.
type GroupReadFormat struct {
	Nr          uint64 /* The number of events */
	TimeEnabled uint64 /* if PERF_FORMAT_TOTAL_TIME_ENABLED */
	TimeRunning uint64 /* if PERF_FORMAT_TOTAL_TIME_RUNNING */
}

type Values struct {
	Value uint64 /* The value of the event */
	ID    uint64 /* if PERF_FORMAT_ID */
}

func (s *PerfEventGroup) readStats(index int, group []PerfEvent, stats *cgroups.Stats) error {
	buf := make([]byte, 24+16*len(group))

	leaderFile := os.NewFile(uintptr(group[0].fd), group[0].String())
	_, err := leaderFile.Read(buf)
	if err != nil {
		return err
	}
	perfData := &GroupReadFormat{}
	reader := bytes.NewReader(buf[:24])
	err = binary.Read(reader, binary.LittleEndian, perfData)
	if err != nil {
		return err
	}
	values := make([]Values, perfData.Nr)
	reader = bytes.NewReader(buf[24:])
	err = binary.Read(reader, binary.LittleEndian, values)
	if err != nil {
		return err
	}

	scalingRatio := 1.0
	if perfData.TimeRunning != 0 && perfData.TimeEnabled != 0 {
		scalingRatio = float64(perfData.TimeEnabled) / float64(perfData.TimeEnabled)
	}
	stats.PerfStats.PerfStat[index] = make([]cgroups.PerfEntry, 0)
	if scalingRatio != float64(0) {
		for i, event := range group {
			stats.PerfStats.PerfStat[index] = append(stats.PerfStats.PerfStat[index], cgroups.PerfEntry{
				ScalingRatio: scalingRatio,
				Value:        uint64(float64(values[i].Value) / scalingRatio),
				Event:        event.String(),
			})
		}
	} else {
		for i, event := range group {
			stats.PerfStats.PerfStat[index] = append(stats.PerfStats.PerfStat[index], cgroups.PerfEntry{
				ScalingRatio: scalingRatio,
				Value:        values[i].Value,
				Event:        event.String(),
			})
		}
	}

	return nil
}

func (s *PerfEventGroup) GetStats(path string, stats *cgroups.Stats) error {
	for index, group := range s.events {
		err := s.readStats(index, group, stats)
		if err != nil {
			return err
		}
	}

	return nil
}
