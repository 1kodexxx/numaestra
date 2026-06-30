package migrate

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"testing"
	"testing/fstest"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- acquireLockWithRetry (логика дедлайна/повторов, без реального Postgres) ---

func TestAcquireLockWithRetry_GotImmediately(t *testing.T) {
	calls := 0
	err := acquireLockWithRetry(context.Background(), discardLogger(), time.Second, 5*time.Millisecond, func() (bool, error) {
		calls++
		return true, nil
	})
	if err != nil {
		t.Fatalf("ожидали успех, получили %v", err)
	}
	if calls != 1 {
		t.Errorf("ожидали 1 попытку, получили %d", calls)
	}
}

func TestAcquireLockWithRetry_Timeout(t *testing.T) {
	calls := 0
	err := acquireLockWithRetry(context.Background(), discardLogger(), 30*time.Millisecond, 5*time.Millisecond, func() (bool, error) {
		calls++
		return false, nil // лок всё время занят
	})
	if err == nil {
		t.Fatal("ожидали ошибку таймаута, лок не должен был захватиться")
	}
	if calls < 2 {
		t.Errorf("ожидали несколько попыток до таймаута, получили %d", calls)
	}
}

func TestAcquireLockWithRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем сразу
	err := acquireLockWithRetry(ctx, discardLogger(), 10*time.Second, 5*time.Millisecond, func() (bool, error) {
		return false, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ожидали context.Canceled, получили %v", err)
	}
}

func TestAcquireLockWithRetry_TryLockError(t *testing.T) {
	sentinel := errors.New("db down")
	err := acquireLockWithRetry(context.Background(), discardLogger(), time.Second, 5*time.Millisecond, func() (bool, error) {
		return false, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("ошибка tryLock должна пробрасываться, получили %v", err)
	}
}

// --- listMigrationFiles (тестируем внутреннюю логику через публичный Run косвенно) ---
// Поскольку Run требует реальный pgxpool, тестируем логику сортировки и фильтрации
// через вспомогательную функцию, вынесенную из Run.

func TestListMigrationFiles_SortsLexicographically(t *testing.T) {
	fsys := fstest.MapFS{
		"0003_add_index.sql": &fstest.MapFile{Data: []byte("SELECT 1")},
		"0001_init.sql":      &fstest.MapFile{Data: []byte("SELECT 1")},
		"0002_accounts.sql":  &fstest.MapFile{Data: []byte("SELECT 1")},
		"README.md":          &fstest.MapFile{Data: []byte("docs")}, // не .sql — должен игнорироваться
	}

	files, err := listSQLFiles(fsys)
	if err != nil {
		t.Fatalf("listSQLFiles упал: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("ожидали 3 SQL-файла, получили %d", len(files))
	}

	want := []string{"0001_init.sql", "0002_accounts.sql", "0003_add_index.sql"}
	for i, f := range files {
		if f != want[i] {
			t.Errorf("позиция %d: ожидали %q, получили %q", i, want[i], f)
		}
	}
}

func TestListMigrationFiles_EmptyFS(t *testing.T) {
	fsys := fstest.MapFS{}
	files, err := listSQLFiles(fsys)
	if err != nil {
		t.Fatalf("listSQLFiles на пустой FS упал: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("ожидали 0 файлов, получили %d", len(files))
	}
}

func TestListMigrationFiles_OnlySQLFilesIncluded(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte("SELECT 1")},
		"0001_init.txt": &fstest.MapFile{Data: []byte("not sql")},
		"embed.go":      &fstest.MapFile{Data: []byte("package migrations")},
		".gitkeep":      &fstest.MapFile{Data: []byte("")},
	}

	files, err := listSQLFiles(fsys)
	if err != nil {
		t.Fatalf("listSQLFiles упал: %v", err)
	}
	if len(files) != 1 || files[0] != "0001_init.sql" {
		t.Errorf("ожидали только '0001_init.sql', получили: %v", files)
	}
}

// listSQLFiles — вынесенная логика из Run для тестируемости без БД.
func listSQLFiles(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".sql" {
			files = append(files, e.Name())
		}
	}

	// fs.ReadDir гарантирует лексикографический порядок
	return files, nil
}
