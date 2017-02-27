#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

function __test_checkpoint_and_restore() {
	requires criu

	opt_detach=""
	opt_predump=""
	while getopts "td" optch; do
		case "$optch" in
			d)
				opt_detach="1"
				;;
			p)
				opt_predump="1"
				;;
		esac
	done

	# criu does not work with external terminals so..
	# setting terminal and root:readonly: to false
	sed -i 's;"terminal": true;"terminal": false;' config.json
	sed -i 's;"readonly": true;"readonly": false;' config.json
	sed -i 's/"sh"/"sh","-c","while :; do date; sleep 1; done"/' config.json

	run_args=()
	if [[ "$opt_detach" ]]; then
		run_args+=("-d")
	fi

	# run busybox
	if [[ "$opt_detach" ]]; then
		runc run "${run_args[@]}" test_busybox
		[ "$status" -eq 0 ]
	else
		(
			runc run "${run_args[@]}" test_busybox
			[ "$status" -eq 0 ]
		) &
	fi

	# check state
	wait_for_container 15 1 test_busybox

	runc state test_busybox
	[ "$status" -eq 0 ]
	[[ "${output}" == *"running"* ]]

	# if you are having problems getting criu to work uncomment the following dump:
	#cat /run/opencontainer/containers/test_busybox/criu.work/dump.log

	restore_args=()
	checkpoint_args=()
	if [[ "$opt_predump" ]]; then
		mkdir parent-dir
		restore_args=("--image-path" "./image-dir")
		checkpoint_args=("--image-path" "./image-dir" "--parent-path" "./parent-dir")

		# do a pre-dump checkpoint
		runc --criu "$CRIU" checkpoint --pre-dump --image-path ./parent-dir test_busybox
		[ "$status" -eq 0 ]

		# busybox should still be running
		runc state test_busybox
		[ "$status" -eq 0 ]
		[[ "${output}" == *"running"* ]]
	fi

	# checkpoint the running container
	mkdir image-dir
	runc --criu "$CRIU" checkpoint "${checkpoint_args[@]}" test_busybox
	[ "$status" -eq 0 ]

	# after checkpoint busybox is no longer running
	runc state test_busybox
	[ "$status" -ne 0 ]

	# restore from checkpoint
	if [[ "$opt_detach" ]]; then
		runc --criu "$CRIU" restore "${restore_args[@]}" "${run_args[@]}" test_busybox
		[ "$status" -eq 0 ]
	else
		(
			runc --criu "$CRIU" restore "${restore_args[@]}" "${run_args[@]}" test_busybox
			[ "$status" -eq 0 ]
		) &
	fi

	# check state
	wait_for_container 15 1 test_busybox

	# busybox should be back up and running
	runc state test_busybox
	[ "$status" -eq 0 ]
	[[ "${output}" == *"running"* ]]

}

@test "checkpoint and restore" {
	__test_checkpoint_and_restore
}

@test "checkpoint and restore [detach]" {
	__test_checkpoint_and_restore -d
}

@test "checkpoint --pre-dump and restore" {
	__test_checkpoint_and_restore -p
}

@test "checkpoint --pre-dump and restore [detach]" {
	__test_checkpoint_and_restore -d -p
}
