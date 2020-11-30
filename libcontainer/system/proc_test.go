package system

import (
	"errors"
	"math/bits"
	"os"
	"reflect"
	"strconv"
	"testing"
)

var procdata = map[string]Stat_t{
	"4902 (gunicorn: maste) S 4885 4902 4902 0 -1 4194560 29683 29929 61 83 78 16 96 17 20 0 1 0 9126532 52965376 1903 18446744073709551615 4194304 7461796 140733928751520 140733928698072 139816984959091 0 0 16781312 137447943 1 0 0 17 3 0 0 9 0 0 9559488 10071156 33050624 140733928758775 140733928758945 140733928758945 140733928759264 0": {
		PID:       4902,
		Name:      "gunicorn: maste",
		State:     'S',
		StartTime: 9126532,
	},
	"9534 (cat) R 9323 9534 9323 34828 9534 4194304 95 0 0 0 0 0 0 0 20 0 1 0 9214966 7626752 168 18446744073709551615 4194304 4240332 140732237651568 140732237650920 140570710391216 0 0 0 0 0 0 0 17 1 0 0 0 0 0 6340112 6341364 21553152 140732237653865 140732237653885 140732237653885 140732237656047 0": {
		PID:       9534,
		Name:      "cat",
		State:     'R',
		StartTime: 9214966,
	},
	"12345 ((ugly )pr()cess() R 9323 9534 9323 34828 9534 4194304 95 0 0 0 0 0 0 0 20 0 1 0 9214966 7626752 168 18446744073709551615 4194304 4240332 140732237651568 140732237650920 140570710391216 0 0 0 0 0 0 0 17 1 0 0 0 0 0 6340112 6341364 21553152 140732237653865 140732237653885 140732237653885 140732237656047 0": {
		PID:       12345,
		Name:      "(ugly )pr()cess(",
		State:     'R',
		StartTime: 9214966,
	},
	"24767 (irq/44-mei_me) S 2 0 0 0 -1 2129984 0 0 0 0 0 0 0 0 -51 0 1 0 8722075 0 0 18446744073709551615 0 0 0 0 0 0 0 2147483647 0 0 0 0 17 1 50 1 0 0 0 0 0 0 0 0 0 0 0": {
		PID:       24767,
		Name:      "irq/44-mei_me",
		State:     'S',
		StartTime: 8722075,
	},
	"0 () I 3 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0": {
		Name:      "",
		State:     'I',
		StartTime: 0,
	},
}

func TestParseStat(t *testing.T) {
	for line, exp := range procdata {
		st, err := parseStat(line)
		if err != nil {
			t.Errorf("input %q, unexpected error %v", line, err)
		} else if !reflect.DeepEqual(st, exp) {
			t.Errorf("input %q, expected %+v, got %+v", line, exp, st)
		}
	}
}

func BenchmarkParseStat(b *testing.B) {
	var (
		st, exp Stat_t
		line    string
		err     error
	)

	for i := 0; i != b.N; i++ {
		for line, exp = range procdata {
			st, err = parseStat(line)
		}
	}
	if err != nil {
		b.Fatal(err)
	}
	if !reflect.DeepEqual(st, exp) {
		b.Fatal("wrong result")
	}
}

func BenchmarkParseRealStat(b *testing.B) {
	var (
		st    Stat_t
		err   error
		total int
	)
	b.StopTimer()
	fd, err := os.Open("/proc")
	if err != nil {
		b.Fatal(err)
	}
	defer fd.Close()

	for i := 0; i != b.N; i++ {
		count := 0
		if _, err := fd.Seek(0, 0); err != nil {
			b.Fatal(err)
		}
		names, err := fd.Readdirnames(-1)
		if err != nil {
			b.Fatal(err)
		}
		for _, n := range names {
			pid, err := strconv.ParseUint(n, 10, bits.UintSize)
			if err != nil {
				continue
			}
			b.StartTimer()
			st, err = Stat(int(pid))
			b.StopTimer()
			if err != nil {
				// Ignore a process that just finished.
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				b.Fatal(err)
			}
			if st.PID != uint(pid) {
				b.Fatalf("pid mismatch, expected %d, got %d", pid, st.PID)
			}
			count++
		}
		total += count
	}
	b.Logf("N: %d, parsed %d pids, last stat: %+v, err: %v", b.N, total, st, err)
}
