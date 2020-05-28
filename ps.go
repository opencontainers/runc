// +build linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	paramLexer map[string]int
	noheader   = []string{
		"no-headings",
		"no-headers",
		"no-heading",
		"noheadings",
		"noheaders",
		"noheading",
		"no-header",
		"noheader",
	}
)

var psCommand = cli.Command{
	Name:      "ps",
	Usage:     "ps displays the processes running inside a container",
	ArgsUsage: `<container-id> [ps options]`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "format, f",
			Value: "table",
			Usage: `select one of: ` + formatOptions,
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}
		rootlessCg, err := shouldUseRootlessCgroupManager(context)
		if err != nil {
			return err
		}
		if rootlessCg {
			logrus.Warn("runc ps may fail if you don't have the full access to cgroups")
		}

		container, err := getContainer(context)
		if err != nil {
			return err
		}

		pids, err := container.Processes()
		if err != nil {
			return err
		}

		switch context.String("format") {
		case "table":
		case "json":
			return json.NewEncoder(os.Stdout).Encode(pids)
		default:
			return errors.New("invalid format option")
		}

		args := context.Args()[1:]
		if len(args) == 0 {
			args = append(args, "-ef")
		}
		paramLexer = UnixPsParamLexer(args)
		addPid := isCustomFormat()
		if addPid {
			// make sure the PID field is shown in the first column
			args = append([]string{"-opid"}, args...)
		}
		if isNoHeader() {
			// make sure there are no --no-header(s) when do ps ...
			// and the Titles will be removed in parsePSOutput if need
			psArgsNew := strings.Join(args, " ")
			for _, v := range noheader {
				psArgsNew = strings.Replace(psArgsNew, " --"+v, "", -1)
				psArgsNew = strings.Replace(psArgsNew, "--"+v, "", -1)
			}

			args = strings.Split(strings.Trim(psArgsNew, " "), " ")
			if len(args) == 1 && args[0] == "" {
				args[0] = "-ef"
			}
		}

		qPids := psPidsArg(pids)
		output, err := exec.Command("ps", append(args, qPids)...).Output()
		if err != nil {
			// some ps options (such as f, -C) can't be used
			// together with q, so retry without it, listing
			// all the processes and applying a filter.
			output, err = exec.Command("ps", args...).Output()
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					// first line of stderr shows why ps failed
					line := bytes.SplitN(ee.Stderr, []byte{'\n'}, 2)
					if len(line) > 0 && len(line[0]) > 0 {
						err = fmt.Errorf(string(line[0]))
					}
				}
				return err
			}
		}

		lines := strings.Split(string(output), "\n")
		if len(lines) == 0 {
			return fmt.Errorf("ps error")
		}
		pidIndex, err := getPidIndex(lines[0])
		if err != nil {
			return err
		}
		if pidIndex < 0 {
			return fmt.Errorf("No PID column found in ps result")
		}
		if !isNoHeader() {
			if addPid {
				fmt.Println(removeFirstColumn(lines[0]))
			} else {
				fmt.Println(lines[0])
			}
		}
		for _, line := range lines[1:] {
			lineArr := fieldsASCII(line)
			if len(lineArr) > pidIndex {
				pid, err := strconv.Atoi(lineArr[pidIndex])
				if err != nil {
					return fmt.Errorf("unexpected pid '%s': %s", lineArr[pidIndex], err)
				}
				if hasPid(pids, pid) {
					if addPid {
						fmt.Println(removeFirstColumn(line))
					} else {
						fmt.Println(line)
					}
				}
			}
		}
		return nil
	},
	SkipArgReorder: true,
}

func getPidIndex(title string) (int, error) {
	titles := strings.Fields(title)

	pidIndex := -1
	for i, name := range titles {
		if name == "PID" {
			return i, nil
		}
	}

	return pidIndex, errors.New("couldn't find PID field in ps output")
}

// fieldsASCII is similar to strings.Fields but only allows ASCII whitespaces
func fieldsASCII(s string) []string {
	return strings.FieldsFunc(s, unicode.IsSpace)
}

// hasPid checks pid is owner to the container
func hasPid(procs []int, pid int) bool {
	for _, p := range procs {
		if int(p) == pid {
			return true
		}
	}
	return false
}

// psPidsArg converts a slice of PIDs to a string consisting
// of comma-separated list of PIDs prepended by "-q".
// For example, psPidsArg([]uint32{1,2,3}) returns "-q1,2,3".
func psPidsArg(pids []int) string {
	b := []byte{'-', 'q'}
	for i, p := range pids {
		b = strconv.AppendUint(b, uint64(p), 10)
		if i < len(pids)-1 {
			b = append(b, ',')
		}
	}
	return string(b)
}

// isCustomFormat checks whether o/-o/--format option is specified.
func isCustomFormat() bool {
	if paramLexer != nil {
		if v, ok := paramLexer["o"]; ok && v == 1 {
			return true
		}
		if v, ok := paramLexer["format"]; ok && v == 1 {
			return true
		}
	}

	return false
}

// isNoHeader checks whether --no-header/--no-headers option is specified.
func isNoHeader() bool {
	if paramLexer != nil {
		for _, v := range noheader {
			if v, ok := paramLexer[v]; ok && v == 1 {
				return true
			}
		}
	}

	return false
}

// removeFirstColumn removes line's first column
func removeFirstColumn(line string) string {
	state := -1
	for idx, c := range line[0:] {
		if state == -1 {
			if !unicode.IsSpace(c) {
				state = 0
			}
		} else if state == 0 {
			if unicode.IsSpace(c) {
				state = 1
			}
		} else if state == 1 {
			if !unicode.IsSpace(c) {
				return line[idx:]
			}
		}
	}
	return line
}
