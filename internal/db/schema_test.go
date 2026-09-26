package db_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/feaziest/kfdesktopbe/internal/db"
)

// mockDBTX implements db.DBTX for testing constructor and interface compliance.
type mockDBTX struct{}

func (m *mockDBTX) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return nil
}

// TestQuerierInterfaceCompliance verifies that *db.Queries implements db.Querier
func TestQuerierInterfaceCompliance(t *testing.T) {
	queries := db.New(&mockDBTX{})
	var _ db.Querier = queries

	if queries == nil {
		t.Fatal("expected queries instance to not be nil")
	}
}

// TestModelStructFields verifies the schema models have all required fields and correct types
func TestModelStructFields(t *testing.T) {
	t.Run("User struct fields", func(t *testing.T) {
		u := db.User{
			ID:          "firebase-uid-123",
			Email:       "user@example.com",
			DisplayName: "Test User",
			AvatarUrl:   "https://example.com/avatar.png",
			CreatedAt:   pgtype.Timestamptz{Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Valid: true},
		}

		val := reflect.ValueOf(u)
		typ := val.Type()

		requiredFields := []string{"ID", "Email", "DisplayName", "AvatarUrl", "CreatedAt", "UpdatedAt"}
		for _, f := range requiredFields {
			if _, ok := typ.FieldByName(f); !ok {
				t.Errorf("missing field %s in db.User struct", f)
			}
		}
	})

	t.Run("Project struct fields", func(t *testing.T) {
		p := db.Project{
			ID:          pgtype.UUID{Valid: true},
			UserID:      "uid-1",
			Title:       "Test Project",
			Description: "Description",
			Color:       "#0D9488",
			Status:      "nextup",
			CreatedAt:   pgtype.Timestamptz{Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Valid: true},
			DeletedAt:   pgtype.Timestamptz{Valid: false},
		}

		val := reflect.ValueOf(p)
		typ := val.Type()

		requiredFields := []string{"ID", "UserID", "Title", "Description", "Color", "Status", "CreatedAt", "UpdatedAt", "DeletedAt"}
		for _, f := range requiredFields {
			if _, ok := typ.FieldByName(f); !ok {
				t.Errorf("missing field %s in db.Project struct", f)
			}
		}
	})

	t.Run("Note struct fields", func(t *testing.T) {
		n := db.Note{
			ID:        pgtype.UUID{Valid: true},
			UserID:    "uid-1",
			ProjectID: pgtype.UUID{Valid: false},
			Title:     "Note title",
			Content:   "Content",
			CreatedAt: pgtype.Timestamptz{Valid: true},
			UpdatedAt: pgtype.Timestamptz{Valid: true},
			DeletedAt: pgtype.Timestamptz{Valid: false},
		}

		val := reflect.ValueOf(n)
		typ := val.Type()

		requiredFields := []string{"ID", "UserID", "ProjectID", "Title", "Content", "CreatedAt", "UpdatedAt", "DeletedAt"}
		for _, f := range requiredFields {
			if _, ok := typ.FieldByName(f); !ok {
				t.Errorf("missing field %s in db.Note struct", f)
			}
		}
	})

	t.Run("NoteLink struct fields", func(t *testing.T) {
		nl := db.NoteLink{
			ID:           pgtype.UUID{Valid: true},
			SourceNoteID: pgtype.UUID{Valid: true},
			TargetNoteID: pgtype.UUID{Valid: true},
			CreatedAt:    pgtype.Timestamptz{Valid: true},
		}

		val := reflect.ValueOf(nl)
		typ := val.Type()

		requiredFields := []string{"ID", "SourceNoteID", "TargetNoteID", "CreatedAt"}
		for _, f := range requiredFields {
			if _, ok := typ.FieldByName(f); !ok {
				t.Errorf("missing field %s in db.NoteLink struct", f)
			}
		}
	})

	t.Run("DailyStat struct fields", func(t *testing.T) {
		ds := db.DailyStat{
			UserID:        "uid-1",
			StatDate:      pgtype.Date{Valid: true},
			ActivityCount: 5,
		}

		val := reflect.ValueOf(ds)
		typ := val.Type()

		requiredFields := []string{"UserID", "StatDate", "ActivityCount"}
		for _, f := range requiredFields {
			if _, ok := typ.FieldByName(f); !ok {
				t.Errorf("missing field %s in db.DailyStat struct", f)
			}
		}
	})
}

// TestQuerierMethodsExist tests via reflection that all required method names exist on db.Querier
func TestQuerierMethodsExist(t *testing.T) {
	querierType := reflect.TypeOf((*db.Querier)(nil)).Elem()

	requiredMethods := []string{
		"UpsertUser",
		"GetUserByID",
		"CreateProject",
		"ListProjectsByUser",
		"GetProjectByID",
		"UpdateProject",
		"SoftDeleteProject",
		"CreateQuicknote",
		"CreateProjectNote",
		"ListNotesByProject",
		"GetNoteByID",
		"UpdateNote",
		"SoftDeleteNote",
		"InsertNoteLink",
		"GetLinkedNotes",
		"IncrementDailyStat",
		"GetDailyStatsByYear",
	}

	for _, methodName := range requiredMethods {
		method, ok := querierType.MethodByName(methodName)
		if !ok {
			t.Errorf("expected Querier to have method %q, but it was not found", methodName)
			continue
		}
		if method.Type.NumIn() < 2 { // receiver + ctx (+ params)
			t.Errorf("expected method %q to accept at least context.Context", methodName)
		}
	}
}
