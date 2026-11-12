package libcontainer

import (
	"errors"
	"reflect"
	"testing"
)

var states = map[containerState]Status{
	&createdState{}:          Created,
	&runningState{}:          Running,
	&restoredState{}:         Running,
	&pausedState{}:           Paused,
	&stoppedState{}:          Stopped,
	&loadedState{s: Running}: Running,
}

func TestStateStatus(t *testing.T) {
	for s, status := range states {
		if s.status() != status {
			t.Fatalf("state returned %s but expected %s", s.status(), status)
		}
	}
}

func testTransitions(t *testing.T, initialState containerState, valid []containerState) {
	validMap := map[reflect.Type]any{}
	for _, validState := range valid {
		validMap[reflect.TypeOf(validState)] = nil
		t.Run(validState.status().String(), func(t *testing.T) {
			if err := initialState.transition(validState); err != nil {
				t.Fatal(err)
			}
		})
	}
	for state := range states {
		if _, ok := validMap[reflect.TypeOf(state)]; ok {
			continue
		}
		t.Run(state.status().String(), func(t *testing.T) {
			err := initialState.transition(state)
			if err == nil {
				t.Fatal("transition should fail")
			}
			if _, ok := errors.AsType[*stateTransitionError](err); !ok {
				t.Fatal("expected stateTransitionError")
			}
		})
	}
}

func TestStoppedStateTransition(t *testing.T) {
	testTransitions(
		t,
		&stoppedState{c: &Container{}},
		[]containerState{
			&stoppedState{},
			&runningState{},
			&restoredState{},
		},
	)
}

func TestPausedStateTransition(t *testing.T) {
	testTransitions(
		t,
		&pausedState{c: &Container{}},
		[]containerState{
			&pausedState{},
			&runningState{},
			&stoppedState{},
		},
	)
}

func TestRestoredStateTransition(t *testing.T) {
	testTransitions(
		t,
		&restoredState{c: &Container{}},
		[]containerState{
			&stoppedState{},
			&runningState{},
		},
	)
}

// TestRestoredStateExitedReportsStopped covers the #5370 regression where
// restoredState.transition accepted stopped/running without updating c.state,
// so a restored container whose process later exited kept reporting Running.
func TestRestoredStateExitedReportsStopped(t *testing.T) {
	c := &Container{}
	c.state = &restoredState{c: c}

	// refreshState() path when the init process is gone: transition to stopped.
	if err := c.state.transition(&stoppedState{c: c}); err != nil {
		t.Fatalf("transition returned error: %v", err)
	}
	if got := c.state.status(); got != Stopped {
		t.Fatalf("restored+exited reports %v, want Stopped", got)
	}
}

func TestRunningStateExitedReportsStopped(t *testing.T) {
	c := &Container{}
	c.state = &runningState{c: c}

	if err := c.state.transition(&stoppedState{c: c}); err != nil {
		t.Fatalf("transition returned error: %v", err)
	}
	if got := c.state.status(); got != Stopped {
		t.Fatalf("running+exited reports %v, want Stopped", got)
	}
}

func TestRunningStateTransition(t *testing.T) {
	testTransitions(
		t,
		&runningState{c: &Container{}},
		[]containerState{
			&stoppedState{},
			&pausedState{},
			&runningState{},
		},
	)
}

func TestCreatedStateTransition(t *testing.T) {
	testTransitions(
		t,
		&createdState{c: &Container{}},
		[]containerState{
			&stoppedState{},
			&pausedState{},
			&runningState{},
			&createdState{},
		},
	)
}
