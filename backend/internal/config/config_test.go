package config

import (
	"reflect"
	"testing"
)

func TestEnvList(t *testing.T) {
	t.Setenv("TEST_ORIGINS", " https://a.com , https://b.com ,, ")
	got := envList("TEST_ORIGINS")
	want := []string{"https://a.com", "https://b.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("envList = %#v, want %#v", got, want)
	}
}

func TestEnvListEmpty(t *testing.T) {
	if got := envList("DEFINITELY_UNSET_VAR"); got != nil {
		t.Fatalf("expected nil for unset var, got %#v", got)
	}
}
