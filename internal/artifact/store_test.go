package artifact

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestImmutableConfinedBuild(t *testing.T) {
	for _, name := range []string{"../secret", "a/../b", "/root", ".env", "a/.git/config", "a\\b", "a%2fb", "_lastapp/config.json", "a//b", "a\n.js"} {
		if ValidPath(name) {
			t.Fatalf("accepted %q", name)
		}
	}
	files := []File{{"index.html", []byte("<h1>hello</h1>")}, {"assets/main.js", []byte("export default 1")}}
	b, e := Inspect(files, "index.html")
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		if e = Store(dir, files, b); e != nil {
			t.Fatal(e)
		}
	}
	if e = Check(dir, b); e != nil {
		t.Fatal(e)
	}
	r, e := Read(dir, b.Hash, "assets/main.js")
	if e != nil {
		t.Fatal(e)
	}
	content, _ := io.ReadAll(r)
	r.Close()
	if string(content) != "export default 1" {
		t.Fatal("wrong content")
	}
	// Even a local filesystem symlink cannot escape the configured root.
	secret := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(secret, []byte("secret"), 0600)
	os.Symlink(secret, filepath.Join(dir, b.Hash, "escape"))
	if r, e = Read(dir, b.Hash, "escape"); e == nil {
		r.Close()
		t.Fatal("symlink escaped artifact root")
	}
	if e = os.WriteFile(filepath.Join(dir, b.Hash, "assets/main.js"), []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if Check(dir, b) == nil {
		t.Fatal("corrupt build accepted")
	}
	if _, e = Inspect(append(files, files[0]), "index.html"); e == nil {
		t.Fatal("duplicate accepted")
	}
}
