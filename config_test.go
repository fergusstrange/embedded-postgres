package embeddedpostgres

import (
	"testing"
)

func TestGetConnectionURL(t *testing.T) {
	config := DefaultConfig().Database("mydb").Username("myuser").Password("mypass")
	expect := "postgresql://myuser:mypass@localhost:5432/mydb"

	if got := config.GetConnectionURL(); got != expect {
		t.Errorf("expected \"%s\" got \"%s\"", expect, got)
	}
}

func TestGetConnectionURLWithUnixSocket(t *testing.T) {
	config := DefaultConfig().Database("mydb").Username("myuser").Password("mypass").WithoutTcp()
	expect := "postgresql://myuser:mypass@:5432/mydb?host=%2Ftmp%2F"

	if got := config.GetConnectionURL(); got != expect {
		t.Errorf("expected \"%s\" got \"%s\"", expect, got)
	}
}

func TestGetConnectionURLWithUnixSocketInCustomDir(t *testing.T) {
	config := DefaultConfig().Database("mydb").Username("myuser").Password("mypass").WithoutTcp().WithUnixSocketDirectory("/path/to/socks")
	expect := "postgresql://myuser:mypass@:5432/mydb?host=%2Fpath%2Fto%2Fsocks"

	if got := config.GetConnectionURL(); got != expect {
		t.Errorf("expected \"%s\" got \"%s\"", expect, got)
	}
}
