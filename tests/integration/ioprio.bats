#!/usr/bin/env bats

load helpers

function setup() {
	setup_debian
}

function teardown() {
	teardown_bundle
}

@test "ioprio_set is applied to process group" {
	# Create a container with a specific I/O priority.
	update_config '.process.ioPriority = {"class": "IOPRIO_CLASS_BE", "priority": 4}'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_ioprio

	# Check the init process.
	runc -0 exec test_ioprio ionice -p 1
	[ "${lines[0]}" = 'best-effort: prio 4' ]

	# Check an exec process, which should derive ioprio from config.json.
	runc -0 exec test_ioprio ionice
	[ "${lines[0]}" = 'best-effort: prio 4' ]

	# Check an exec with a priority taken from process.json,
	# which should override the ioprio in config.json.
	proc='
{
	"terminal": false,
	"ioPriority": {
		"class": "IOPRIO_CLASS_IDLE"
	},
	"args": [ "/usr/bin/ionice" ],
	"cwd": "/"
}'
	runc -0 exec --process <(echo "$proc") test_ioprio
	[ "${lines[0]}" = 'idle' ]
}
