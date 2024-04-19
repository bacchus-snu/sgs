package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/bacchus-snu/sgs/model"
	"github.com/bacchus-snu/sgs/model/test"
)

func TestPostgres(t *testing.T) {
	dbURL := os.Getenv("SGS_TEST_DBURL")
	if dbURL == "" {
		t.Skip("SGS_TEST_DBURL is not set")
	}

	test.TestWorkspace(t, func() model.WorkspaceService {
		// reset the DB on every test
		mig, err := openMigrations(dbURL)
		if err != nil {
			t.Fatalf("openMigrations() = %v", err)
		}
		err = mig.Drop()
		mig.Close()
		if err != nil {
			t.Fatalf("mig.Drop() = %v", err)
		}

		repo, err := New(context.Background(), Config{dbURL})
		if err != nil {
			t.Fatalf("New() = %v", err)
		}
		t.Cleanup(func() { repo.Close() })

		return repo.Workspaces()
	})
}
