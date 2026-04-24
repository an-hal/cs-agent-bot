package mockoutbox

import (
	"testing"
)

func TestRecord_AssignsIDAndTimestamp(t *testing.T) {
	o := New(10)
	r := o.Record(ProviderClaude, "extract", map[string]any{"x": 1}, map[string]any{"y": 2}, "success", "")
	if r.ID == "" {
		t.Error("expected non-empty id")
	}
	if r.Timestamp.IsZero() {
		t.Error("expected timestamp set")
	}
	if r.Provider != ProviderClaude {
		t.Errorf("provider mismatch: %q", r.Provider)
	}
}

func TestList_NewestFirstAndLimited(t *testing.T) {
	o := New(10)
	for i := 0; i < 5; i++ {
		o.Record(ProviderHaloAI, "send", nil, nil, "success", "")
	}
	rs := o.List("", 3)
	if len(rs) != 3 {
		t.Fatalf("expected 3, got %d", len(rs))
	}
	// Newest first — the last inserted should be first.
	for i := 1; i < len(rs); i++ {
		if rs[i-1].Timestamp.Before(rs[i].Timestamp) {
			t.Error("expected newest-first order")
		}
	}
}

func TestList_ProviderFilter(t *testing.T) {
	o := New(10)
	o.Record(ProviderClaude, "a", nil, nil, "success", "")
	o.Record(ProviderFireflies, "b", nil, nil, "success", "")
	o.Record(ProviderClaude, "c", nil, nil, "success", "")

	rs := o.List(ProviderClaude, 0)
	if len(rs) != 2 {
		t.Errorf("expected 2 claude records, got %d", len(rs))
	}
}

func TestRecord_RingBufferEvictsOldest(t *testing.T) {
	o := New(3)
	for i := 0; i < 5; i++ {
		o.Record(ProviderSMTP, "send", map[string]any{"seq": i}, nil, "success", "")
	}
	rs := o.List("", 0)
	if len(rs) != 3 {
		t.Fatalf("capacity=3, expected 3 records, got %d", len(rs))
	}
	// After ring eviction the records 2,3,4 survive. Newest-first: 4,3,2.
	if seq, _ := rs[0].Payload["seq"].(int); seq != 4 {
		t.Errorf("expected newest seq=4, got %v", rs[0].Payload["seq"])
	}
}

func TestClear_ByProviderAndAll(t *testing.T) {
	o := New(10)
	o.Record(ProviderClaude, "a", nil, nil, "success", "")
	o.Record(ProviderSMTP, "b", nil, nil, "success", "")
	o.Record(ProviderClaude, "c", nil, nil, "success", "")

	if n := o.Clear(ProviderClaude); n != 2 {
		t.Errorf("expected 2 cleared, got %d", n)
	}
	if rs := o.List("", 0); len(rs) != 1 {
		t.Errorf("expected 1 left, got %d", len(rs))
	}
	if n := o.Clear(""); n != 1 {
		t.Errorf("expected 1 cleared-all, got %d", n)
	}
	if rs := o.List("", 0); len(rs) != 0 {
		t.Errorf("expected empty, got %d", len(rs))
	}
}
