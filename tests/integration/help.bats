#!/usr/bin/env bats

load helpers

@test "runc -h" {
  run "$RUNC" -h
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ NAME:+ ]]
  [[ ${lines[1]} =~ runc\ '-'\ Open\ Container\ Initiative\ runtime+ ]]

  run "$RUNC" --help
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ NAME:+ ]]
  [[ ${lines[1]} =~ runc\ '-'\ Open\ Container\ Initiative\ runtime+ ]]
}

@test "runc command -h" {
  run "$RUNC" checkpoint -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ checkpoint+ ]]

  run "$RUNC" delete -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ delete+ ]]

  run "$RUNC" events -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ events+ ]]

  run "$RUNC" exec -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ exec+ ]]

  run "$RUNC" kill -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ kill+ ]]

  run "$RUNC" list -h
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ NAME:+ ]]
  [[ ${lines[1]} =~ runc\ list+ ]]

  run "$RUNC" list --help
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ NAME:+ ]]
  [[ ${lines[1]} =~ runc\ list+ ]]

  run "$RUNC" pause -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ pause+ ]]

  run "$RUNC" restore -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ restore+ ]]

  run "$RUNC" resume -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ resume+ ]]

  run "$RUNC" spec -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ spec+ ]]

  run "$RUNC" start -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ start+ ]]

  run "$RUNC" state -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ state+ ]]

  run "$RUNC" delete -h
  [ "$status" -eq 0 ]
  [[ ${lines[1]} =~ runc\ delete+ ]]

}

@test "runc foo -h" {
  run "$RUNC" foo -h
  [ "$status" -ne 0 ]
  [[ "${output}" == *"No help topic for 'foo'"* ]]
}
