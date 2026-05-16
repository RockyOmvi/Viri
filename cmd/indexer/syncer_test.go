package main

import (
	"testing"
)

func TestParseHexUint64(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"0x0", 0},
		{"0x1", 1},
		{"0xff", 255},
		{"0x64", 100},
		{"0xABCDEF", 11259375},
		{"0X1234", 4660},
		{"", 0},
		{"nothex", 0},
		{"0x", 0},
		{"0xFFFFFFFFFFFFFFFF", 18446744073709551615},
	}

	for _, tt := range tests {
		got := parseHexUint64(tt.input)
		if got != tt.want {
			t.Errorf("parseHexUint64(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestGetString(t *testing.T) {
	m := map[string]interface{}{
		"foo":    "bar",
		"num":    42,
		"empty":  "",
		"nested": map[string]interface{}{"a": "b"},
	}

	if got := getString(m, "foo"); got != "bar" {
		t.Errorf("getString(foo) = %q, want %q", got, "bar")
	}
	if got := getString(m, "missing"); got != "" {
		t.Errorf("getString(missing) = %q, want empty", got)
	}
	if got := getString(m, "empty"); got != "" {
		t.Errorf("getString(empty) = %q, want empty", got)
	}
	if got := getString(m, "num"); got != "42" {
		t.Errorf("getString(num) = %q, want %q", got, "42")
	}
	if got := getString(nil, "foo"); got != "" {
		t.Errorf("getString(nil, foo) = %q, want empty", got)
	}
}

func TestParsePageParamsDefaults(t *testing.T) {
	r := mustBuildRequest("/api/v1/test")
	page, limit := parsePageParams(r, 1, 20)
	if page != 1 {
		t.Errorf("default page = %d, want 1", page)
	}
	if limit != 20 {
		t.Errorf("default limit = %d, want 20", limit)
	}
}

func TestParsePageParamsCustom(t *testing.T) {
	r := mustBuildRequest("/api/v1/test?page=3&limit=10")
	page, limit := parsePageParams(r, 1, 20)
	if page != 3 {
		t.Errorf("page = %d, want 3", page)
	}
	if limit != 10 {
		t.Errorf("limit = %d, want 10", limit)
	}
}

func TestParsePageParamsInvalid(t *testing.T) {
	r := mustBuildRequest("/api/v1/test?page=-1&limit=999&invalid=abc")
	page, limit := parsePageParams(r, 1, 20)
	if page != 1 {
		t.Errorf("invalid page should fallback to default, got %d", page)
	}
	if limit != 20 {
		t.Errorf("limit > 100 should fallback to default, got %d", limit)
	}
}

func TestParsePageParamsZero(t *testing.T) {
	r := mustBuildRequest("/api/v1/test?page=0&limit=0")
	page, limit := parsePageParams(r, 1, 20)
	if page != 1 {
		t.Errorf("page=0 should use default, got %d", page)
	}
	if limit != 20 {
		t.Errorf("limit=0 should use default, got %d", limit)
	}
}

func TestStoredBlockDefaultValues(t *testing.T) {
	b := StoredBlock{}
	if b.Hash != "" {
		t.Errorf("new StoredBlock should have empty Hash")
	}
	if b.Number != 0 {
		t.Errorf("new StoredBlock should have Number=0")
	}
	if b.TxCount != 0 {
		t.Errorf("new StoredBlock should have TxCount=0")
	}
}

func TestStoredTxDefaults(t *testing.T) {
	tx := StoredTx{}
	if tx.Status != "" {
		t.Errorf("new StoredTx should have empty Status")
	}
	if tx.From != "" {
		t.Errorf("new StoredTx should have empty From")
	}
	if tx.BlockNumber != 0 {
		t.Errorf("new StoredTx should have BlockNumber=0")
	}
}

func TestStoredReceiptDefaults(t *testing.T) {
	r := StoredReceipt{}
	if r.Status != "" {
		t.Errorf("new StoredReceipt should have empty Status")
	}
	if r.Logs != nil {
		t.Errorf("new StoredReceipt Logs should be nil")
	}
}

func TestStoredLogDefaults(t *testing.T) {
	l := StoredLog{}
	if l.Address != "" {
		t.Errorf("new StoredLog should have empty Address")
	}
	if l.Topics != nil {
		t.Errorf("new StoredLog Topics should be nil")
	}
}

func TestSyncStateDefaults(t *testing.T) {
	s := SyncState{}
	if s.ID != "" {
		t.Errorf("new SyncState should have empty ID")
	}
	if s.LastBlock != 0 {
		t.Errorf("new SyncState should have LastBlock=0")
	}
}
