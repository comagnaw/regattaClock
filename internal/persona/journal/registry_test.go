package journal

import (
	"path/filepath"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona"
)

// sessionFor mirrors internal/persona/store's test helper of the same name -
// journal has no import of store to reuse it from, so it's duplicated here at
// the same small size rather than introducing a dependency just for a test
// fixture.
func sessionFor(t *testing.T, id, root string) persona.Session {
	t.Helper()
	def, ok := persona.ByID(id)
	if !ok {
		t.Fatalf("unknown persona %q", id)
	}
	return persona.Session{Definition: def, Root: root}
}

func configureTempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	Configure(dir)
	t.Cleanup(func() { Configure("") })
	return dir
}

func TestFor_MemoizesByWritePath(t *testing.T) {
	configureTempRoot(t)
	sess := sessionFor(t, "pst", t.TempDir())

	m1, err := For(sess, "regatta-a")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	t.Cleanup(m1.Close)

	m2, err := For(sess, "regatta-a")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if m1 != m2 {
		t.Error("expected the same Manager for the same WritePath")
	}
}

func TestFor_DifferentTeams_DifferentManagers(t *testing.T) {
	configureTempRoot(t)
	root := t.TempDir()

	primary, err := For(sessionFor(t, "pst", root), "regatta-a")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	t.Cleanup(primary.Close)

	secondary, err := For(sessionFor(t, "sst", root), "regatta-a")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	t.Cleanup(secondary.Close)

	if primary == secondary {
		t.Error("expected distinct Managers for distinct teams")
	}
}

func TestFor_PersonaWithNoWritePath_ReturnsError(t *testing.T) {
	configureTempRoot(t)
	sess := persona.Session{Definition: persona.AwardsDefinition, Root: t.TempDir()}

	if _, err := For(sess, "regatta-a"); err == nil {
		t.Error("expected an error for a persona with no write path")
	}
}

func TestFor_LocalPathIsNamespacedByRootTeamAndRole(t *testing.T) {
	dir := configureTempRoot(t)
	root := t.TempDir()
	sess := sessionFor(t, "pft", root)

	m, err := For(sess, "regatta-a")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	t.Cleanup(m.Close)

	want := filepath.Join(dir, rootNamespace(root), "primary", "finish.pending.json")
	if m.localPath != want {
		t.Errorf("expected local path %q, got %q", want, m.localPath)
	}
}

func TestFor_DifferentRoots_DifferentLocalPaths(t *testing.T) {
	dir := configureTempRoot(t)

	// Same regattaKey on purpose - what must not collide here is the folder
	// (Session.Root), the same way two unit tests that both leave regattaKey
	// at its zero value must not collide either.
	m1, err := For(sessionFor(t, "pst", t.TempDir()), "regatta-a")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	t.Cleanup(m1.Close)

	root2 := t.TempDir()
	m2, err := For(sessionFor(t, "pst", root2), "regatta-a")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	t.Cleanup(m2.Close)

	if m1.localPath == m2.localPath {
		t.Error("expected different Session.Root folders to namespace to different local paths")
	}
	if filepath.Dir(filepath.Dir(m2.localPath)) != filepath.Join(dir, rootNamespace(root2)) {
		t.Errorf("expected root2's local path under %q, got %q", filepath.Join(dir, rootNamespace(root2)), m2.localPath)
	}
}

func TestRootDir_DefaultsToOSCacheDirWhenUnconfigured(t *testing.T) {
	Configure("")
	dir, err := rootDir()
	if err != nil {
		t.Fatalf("rootDir: %v", err)
	}
	if filepath.Base(dir) != "journal" {
		t.Errorf("expected the default root to end in .../journal, got %q", dir)
	}
}
