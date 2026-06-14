package opentdb

import (
	"strings"
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// The client's HTTP behaviour is covered in opentdb_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "opentdb" {
		t.Errorf("Scheme = %q, want opentdb", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "opentdb" {
		t.Errorf("Identity.Binary = %q, want opentdb", info.Identity.Binary)
	}
}

func TestClassifyNumeric(t *testing.T) {
	typ, id, err := Domain{}.Classify("42")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if typ != "category" {
		t.Errorf("typ = %q, want category", typ)
	}
	if id != "42" {
		t.Errorf("id = %q, want 42", id)
	}
}

func TestClassifyQuery(t *testing.T) {
	typ, id, err := Domain{}.Classify("science")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if typ != "query" {
		t.Errorf("typ = %q, want query", typ)
	}
	if id != "science" {
		t.Errorf("id = %q, want science", id)
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") should return error")
	}
}

func TestLocateCategory(t *testing.T) {
	got, err := Domain{}.Locate("category", "18")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if !strings.Contains(got, "browse.php") {
		t.Errorf("Locate(category,18) = %q, want URL with browse.php", got)
	}
	if !strings.Contains(got, "18") {
		t.Errorf("Locate(category,18) = %q, want URL containing 18", got)
	}
}

func TestLocateQuery(t *testing.T) {
	got, err := Domain{}.Locate("query", "science")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got == "" {
		t.Error("Locate returned empty URL")
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate with unknown type should return error")
	}
}
