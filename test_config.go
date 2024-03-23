package embeddedpostgres

import "testing"

func TestGetConnectionURL(t *testing.T) {
	config := DefaultConfig().Database("mydb").Username("myuser").Password("mypass")
	expect := "postgresql://myuser:mypass@localhost:5432/mydb"

	if got := config.GetConnectionURL(); got != expect {
		t.Errorf("expected \"%s\" got \"%s\"", expect, got)
	}
}
