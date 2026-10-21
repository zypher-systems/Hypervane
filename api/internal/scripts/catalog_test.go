package scripts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

var fieldTypes = map[string]bool{
	"node": true, "target-node": true, "storage": true, "backup-storage": true, "guest": true,
	"qemu-guest": true, "template": true, "source": true, "mode": true, "number": true,
	"text": true, "keys": true, "bool": true,
}

// Proxmox only imports these names (pve-storage UPLOAD_IMPORT_EXT_RE_1).
var importName = regexp.MustCompile(`^[A-Za-z0-9_.+-]+\.(qcow2|raw|vmdk)$`)

func TestCatalogIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Catalog {
		if seen[r.ID] {
			t.Fatalf("duplicate recipe %s", r.ID)
		}
		seen[r.ID] = true
		if r.Title == "" || r.Summary == "" || r.Category == "" || r.Kind == "" {
			t.Fatalf("%s is missing a title, summary, category or kind", r.ID)
		}
		names := map[string]bool{}
		for _, f := range r.Fields {
			if !fieldTypes[f.Type] {
				t.Fatalf("%s field %s has unknown type %q", r.ID, f.Name, f.Type)
			}
			if names[f.Name] {
				t.Fatalf("%s repeats field %s", r.ID, f.Name)
			}
			names[f.Name] = true
		}
		if r.Kind != KindCloudTemplate {
			continue
		}
		img := r.Image
		if img == nil || !strings.HasPrefix(img.URL, "https://") || !importName.MatchString(img.Filename) {
			t.Fatalf("%s image %+v: needs an https URL and a .qcow2/.raw/.vmdk file name", r.ID, img)
		}
		if img.Index != "" {
			re, err := regexp.Compile(img.Pattern)
			if err != nil || !re.MatchString(img.Filename) {
				t.Fatalf("%s: the pinned file must match its own pattern", r.ID)
			}
			if img.URL != img.Index+img.Filename {
				t.Fatalf("%s: the pinned URL must be Index + Filename", r.ID)
			}
		}
		if checkName(r.DefaultName) != nil {
			t.Fatalf("%s default name %q", r.ID, r.DefaultName)
		}
	}
}

// Every recipe has a planner: an unknown kind is the only error that isn't
// about the inputs or the cluster.
func TestEveryRecipeHasAPlanner(t *testing.T) {
	_, c := newFake(t)
	for _, r := range Catalog {
		_, err := Prepare(context.Background(), c, nil, r, Params{})
		if err != nil && strings.Contains(err.Error(), "unknown recipe kind") {
			t.Fatalf("%s: %v", r.ID, err)
		}
	}
}

func TestPublicListsTheWholeLibrary(t *testing.T) {
	if n := len(Public()); n < 20 {
		t.Fatalf("expected the whole library, got %d recipes", n)
	}
}

func TestVersionOrder(t *testing.T) {
	cases := [][2]string{
		{"generic_alpine-3.24.9-x86_64-bios-cloudinit-r0.qcow2", "generic_alpine-3.24.10-x86_64-bios-cloudinit-r0.qcow2"},
		{"Fedora-Cloud-Base-Generic-44-1.7.x86_64.qcow2", "Fedora-Cloud-Base-Generic-44-1.10.x86_64.qcow2"},
		{"generic_alpine-3.24.1-x86_64-bios-cloudinit-r0.qcow2", "generic_alpine-3.24.1-x86_64-bios-cloudinit-r1.qcow2"},
	}
	for _, c := range cases {
		if !versionLess(c[0], c[1]) || versionLess(c[1], c[0]) {
			t.Fatalf("%s should sort before %s", c[0], c[1])
		}
	}
}

func TestImagesPickTheNewestBuildAndFallBack(t *testing.T) {
	listing := `<a href="generic_alpine-3.24.1-x86_64-bios-cloudinit-r0.qcow2">x</a>
<a href="generic_alpine-3.24.10-x86_64-bios-cloudinit-r0.qcow2">x</a>
<a href="generic_alpine-3.24.9-x86_64-bios-cloudinit-r0.qcow2">x</a>
<a href="generic_alpine-3.24.10-x86_64-uefi-cloudinit-r0.qcow2">x</a>`
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/gone/" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(listing))
	}))
	defer srv.Close()
	im := NewImages()
	img := Image{
		URL:      srv.URL + "/v3.24/generic_alpine-3.24.1-x86_64-bios-cloudinit-r0.qcow2",
		Filename: "generic_alpine-3.24.1-x86_64-bios-cloudinit-r0.qcow2",
		Index:    srv.URL + "/v3.24/",
		Pattern:  `generic_alpine-3\.24\.[0-9]+-x86_64-bios-cloudinit-r[0-9]+\.qcow2`,
	}
	got := im.Resolve(context.Background(), img)
	if got.Pinned || got.Filename != "generic_alpine-3.24.10-x86_64-bios-cloudinit-r0.qcow2" || got.URL != img.Index+got.Filename {
		t.Fatalf("resolved %+v", got)
	}
	im.Resolve(context.Background(), img)
	if hits != 1 {
		t.Fatalf("the listing should be cached, fetched %d times", hits)
	}
	img.Index = srv.URL + "/gone/"
	if got := im.Resolve(context.Background(), img); !got.Pinned || got.Filename != img.Filename || got.URL != img.URL {
		t.Fatalf("an unreadable listing must fall back to the pinned build: %+v", got)
	}
	// Without a resolver the recipe's own link is used.
	if got := (*Images)(nil).Resolve(context.Background(), img); got.URL != img.URL {
		t.Fatalf("nil resolver %+v", got)
	}
}

// The page reads fields as a list, so a recipe without inputs sends [].
func TestPublicNeverSendsNullFields(t *testing.T) {
	raw, err := json.Marshal(Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"fields":null`) {
		t.Fatal("a recipe sent fields: null")
	}
}
