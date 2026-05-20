package database

import "testing"

func TestLogicalDatabaseName(t *testing.T) {
	got := LogicalDatabaseName(" router ")
	if got != "router" {
		t.Fatalf("LogicalDatabaseName() = %q, want %q", got, "router")
	}
}
