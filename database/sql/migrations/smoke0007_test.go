package migrations

import (
	"testing"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestScaleSetCreateFailureBudgetMigrationOnExistingDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := db.Exec("CREATE TABLE scale_sets (id integer PRIMARY KEY, name text)").Error; err != nil {
		t.Fatalf("failed to create scale_sets: %v", err)
	}

	var target []*gormigrate.Migration
	for _, m := range All() {
		if m.ID == "0007_scaleset_create_failure_budget" {
			target = append(target, m)
		}
	}
	if len(target) != 1 {
		t.Fatalf("expected to find 0007 migration, got %d", len(target))
	}

	m := gormigrate.New(db, gormigrate.DefaultOptions, target)
	if err := m.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&scaleSet0007{}, "max_create_attempts") {
		t.Fatal("scale_sets table is missing max_create_attempts")
	}
	if !db.Migrator().HasColumn(&scaleSet0007{}, "create_failures") {
		t.Fatal("scale_sets table is missing create_failures")
	}
}
