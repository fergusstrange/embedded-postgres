package embeddedpostgres

import "testing"

// Both libpq authentication forms must work, including on Windows where Go's
// restricted-token subprocess defaults otherwise discard PGPASSFILE.
func TestPasswordRoundTrip(t *testing.T) {
	t.Setenv("PGPASSWORD", "incorrect inherited password")
	c := integrationConfig(t).Username("test-role").Password("fixture:@ /\\ password")
	pg := NewDatabase(c)
	if e := pg.Start(); e != nil {
		t.Fatal(e)
	}
	defer pg.Close()
	for _, withPassword := range []bool{false, true} {
		connection := withoutPassword(pg.ConnectionURL())
		if withPassword {
			connection = pg.ConnectionURL()
		}
		if _, e := pg.command(t.Context(), pg.active, "psql", "-X", "-w", "-d", connection, "-Atc", "SELECT 1"); e != nil {
			t.Errorf("authentication form with URI password=%v failed", withPassword)
		}
	}
}
