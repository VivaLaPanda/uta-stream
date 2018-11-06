package migrator

import "testing"

func TestMigrator(t *testing.T) {
	Migrate("cache.db", "metadata.db")
}
