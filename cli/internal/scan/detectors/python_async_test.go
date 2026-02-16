package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestPythonAsyncDetectsSyncInAsync(t *testing.T) {
	tmp := t.TempDir()

	// Python project marker.
	os.WriteFile(filepath.Join(tmp, "requirements.txt"), []byte("fastapi\nrequests\n"), 0644)

	// Handler with async def calling blocking requests.get().
	handler := `import requests
from fastapi import FastAPI

app = FastAPI()

@app.get("/data")
async def get_data():
    resp = requests.get("https://api.example.com/data")
    return resp.json()

async def fetch_with_sleep():
    import time
    time.sleep(5)
    return "done"
`
	os.WriteFile(filepath.Join(tmp, "handler.py"), []byte(handler), 0644)

	det := PythonAsync{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected sections for sync calls in async functions")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if ts.Origin != model.OriginAuto {
		t.Errorf("origin = %q, want %q", ts.Origin, model.OriginAuto)
	}
	if ts.Title != "Sync Calls in Async Functions" {
		t.Errorf("title = %q, want %q", ts.Title, "Sync Calls in Async Functions")
	}
}

func TestPythonAsyncCleanProject(t *testing.T) {
	tmp := t.TempDir()

	// Python project with proper async code (no blocking calls).
	os.WriteFile(filepath.Join(tmp, "requirements.txt"), []byte("fastapi\nhttpx\n"), 0644)

	handler := `import httpx
from fastapi import FastAPI

app = FastAPI()

@app.get("/data")
async def get_data():
    async with httpx.AsyncClient() as client:
        resp = await client.get("https://api.example.com/data")
        return resp.json()
`
	os.WriteFile(filepath.Join(tmp, "handler.py"), []byte(handler), 0644)

	det := PythonAsync{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean async project, got %d", len(sections))
	}
}

func TestPythonAsyncSkipsNonPython(t *testing.T) {
	tmp := t.TempDir()
	// Empty dir — no Python markers.

	det := PythonAsync{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if sections != nil {
		t.Errorf("expected nil sections for non-Python project, got %v", sections)
	}
}
