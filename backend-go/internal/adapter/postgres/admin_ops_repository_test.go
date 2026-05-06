package postgres

import "testing"

func TestNewAdminOpsRepositoriesRejectNilQuerier(t *testing.T) {
	if _, err := NewAdminStatsRepository(nil); err == nil {
		t.Fatal("NewAdminStatsRepository(nil) error = nil, want error")
	}
	if _, err := NewAdminSettingsRepository(nil); err == nil {
		t.Fatal("NewAdminSettingsRepository(nil) error = nil, want error")
	}
	if _, err := NewSecurityLogRepository(nil); err == nil {
		t.Fatal("NewSecurityLogRepository(nil) error = nil, want error")
	}
}
