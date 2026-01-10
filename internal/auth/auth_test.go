package auth

import "testing"

func TestHandler_CheckAndGrant(t *testing.T) {
	h := NewHandler()
	req := Request{Action: ActionFSWrite, Resource: "config.yaml"}

	// 1. Initially should be "ask"
	if h.Check(req) != DecisionAsk {
		t.Errorf("expected ask, got %v", h.Check(req))
	}

	// 2. Grant for session
	h.Grant(req, DecisionAllow, DurationSession)
	if h.Check(req) != DecisionAllow {
		t.Errorf("expected allow, got %v", h.Check(req))
	}

	// 3. Different resource should still ask
	req2 := Request{Action: ActionFSWrite, Resource: "other.txt"}
	if h.Check(req2) != DecisionAsk {
		t.Errorf("expected ask for different resource, got %v", h.Check(req2))
	}

	// 4. Permanent grant
	h.Grant(req2, DecisionDeny, DurationPermanent)
	if h.Check(req2) != DecisionDeny {
		t.Errorf("expected deny, got %v", h.Check(req2))
	}
}

