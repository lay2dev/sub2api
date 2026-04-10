package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsSystemLogsWhere_WithClientRequestIDAndUserID(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	userID := int64(12)
	accountID := int64(34)

	filter := &service.OpsSystemLogFilter{
		StartTime:       &start,
		EndTime:         &end,
		Level:           "warn",
		Component:       "http.access",
		RequestID:       "req-1",
		ClientRequestID: "creq-1",
		UserID:          &userID,
		AccountID:       &accountID,
		Platform:        "openai",
		Model:           "gpt-5",
		Query:           "timeout",
	}

	where, args, hasConstraint := buildOpsSystemLogsWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 11 {
		t.Fatalf("args len = %d, want 11", len(args))
	}
	if !contains(where, "COALESCE(l.client_request_id,'') = $") {
		t.Fatalf("where should include client_request_id condition: %s", where)
	}
	if !contains(where, "l.user_id = $") {
		t.Fatalf("where should include user_id condition: %s", where)
	}
}

func TestBuildOpsSystemLogsCleanupWhere_RequireConstraint(t *testing.T) {
	where, args, hasConstraint := buildOpsSystemLogsCleanupWhere(&service.OpsSystemLogCleanupFilter{})
	if hasConstraint {
		t.Fatalf("expected hasConstraint=false")
	}
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 0 {
		t.Fatalf("args len = %d, want 0", len(args))
	}
}

func TestBuildOpsSystemLogsCleanupWhere_WithClientRequestIDAndUserID(t *testing.T) {
	userID := int64(9)
	filter := &service.OpsSystemLogCleanupFilter{
		ClientRequestID: "creq-9",
		UserID:          &userID,
	}

	where, args, hasConstraint := buildOpsSystemLogsCleanupWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if len(args) != 2 {
		t.Fatalf("args len = %d, want 2", len(args))
	}
	if !contains(where, "COALESCE(l.client_request_id,'') = $") {
		t.Fatalf("where should include client_request_id condition: %s", where)
	}
	if !contains(where, "l.user_id = $") {
		t.Fatalf("where should include user_id condition: %s", where)
	}
}

func TestBuildOpsSystemLogsWhere_WithCryptoAdapterName(t *testing.T) {
	filter := &service.OpsSystemLogFilter{
		CryptoAdapterName: "gecko",
	}

	where, args, hasConstraint := buildOpsSystemLogsWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if got := args[0]; got != "%gecko%" {
		t.Fatalf("arg[0] = %v, want %%gecko%%", got)
	}
	if !contains(where, "jsonb_array_elements_text") {
		t.Fatalf("where should inspect crypto_adapter_names json array: %s", where)
	}
}

func TestBuildOpsSystemLogsCleanupWhere_WithCryptoAdapterName(t *testing.T) {
	filter := &service.OpsSystemLogCleanupFilter{
		CryptoAdapterName: "defillama",
	}

	where, args, hasConstraint := buildOpsSystemLogsCleanupWhere(filter)
	if !hasConstraint {
		t.Fatalf("expected hasConstraint=true")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if got := args[0]; got != "%defillama%" {
		t.Fatalf("arg[0] = %v, want %%defillama%%", got)
	}
	if !contains(where, "jsonb_array_elements_text") {
		t.Fatalf("where should inspect crypto_adapter_names json array: %s", where)
	}
}

func contains(s string, sub string) bool {
	return strings.Contains(s, sub)
}
