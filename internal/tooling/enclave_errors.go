package tooling

import "errors"

var (
	// ErrNeedsApproval signals that a tool/command requires explicit user approval.
	ErrNeedsApproval = errors.New("security: approval required")
)

// ApprovalRequest is a structured description of what needs approval.
// It is intended to be shown to the user.
type ApprovalRequest struct {
	Key         string `json:"key"`
	ToolName    string `json:"tool_name"`
	Summary     string `json:"summary"`
	Risk        string `json:"risk"`        // low|medium|high|blocked
	Suggestion  string `json:"suggestion"`  // how user can respond
	ArgsPreview string `json:"args_preview"`// short preview
}

// NeedsApprovalError wraps an ApprovalRequest.
type NeedsApprovalError struct {
	Request ApprovalRequest
}

func (e *NeedsApprovalError) Error() string {
	return ErrNeedsApproval.Error() + ": " + e.Request.Summary
}

func (e *NeedsApprovalError) Unwrap() error { return ErrNeedsApproval }
