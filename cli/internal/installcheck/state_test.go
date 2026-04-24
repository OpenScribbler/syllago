package installcheck

import "testing"

func TestStateTypes_ZeroValue(t *testing.T) {
	t.Parallel()
	var s State
	if s != StateFresh {
		t.Errorf("zero-value State: got %d, want StateFresh (%d)", s, StateFresh)
	}
	var r Reason
	if r != ReasonNone {
		t.Errorf("zero-value Reason: got %d, want ReasonNone (%d)", r, ReasonNone)
	}
	var pts PerTargetState
	if pts.State != StateFresh || pts.Reason != ReasonNone {
		t.Errorf("zero-value PerTargetState: got %+v, want {StateFresh, ReasonNone}", pts)
	}
	var rk RecordKey
	if rk.LibraryID != "" || rk.TargetFile != "" {
		t.Errorf("zero-value RecordKey: got %+v, want empty", rk)
	}
	var vr VerificationResult
	if vr.PerRecord != nil || vr.MatchSet != nil || vr.Warnings != nil {
		t.Errorf("zero-value VerificationResult: got %+v, want nil maps/slices", vr)
	}
}
