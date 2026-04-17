package capmon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFollowRedirectChain_NoRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	chain, err := FollowRedirectChain(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FollowRedirectChain: %v", err)
	}
	if chain.FinalURL != srv.URL {
		t.Errorf("FinalURL = %q, want %q", chain.FinalURL, srv.URL)
	}
	if len(chain.Hops) != 0 {
		t.Errorf("Hops = %d, want 0", len(chain.Hops))
	}
	if !chain.Permanent {
		t.Error("Permanent = false, want true for empty chain")
	}
}

func TestFollowRedirectChain_SingleMovedPermanently(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/final")
		w.WriteHeader(http.StatusMovedPermanently)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	chain, err := FollowRedirectChain(context.Background(), srv.URL+"/start")
	if err != nil {
		t.Fatalf("FollowRedirectChain: %v", err)
	}
	if chain.FinalURL != srv.URL+"/final" {
		t.Errorf("FinalURL = %q, want %q", chain.FinalURL, srv.URL+"/final")
	}
	if len(chain.Hops) != 1 {
		t.Fatalf("Hops = %d, want 1", len(chain.Hops))
	}
	if chain.Hops[0].Status != http.StatusMovedPermanently {
		t.Errorf("Hops[0].Status = %d, want 301", chain.Hops[0].Status)
	}
	if !chain.Permanent {
		t.Error("Permanent = false, want true for 301 chain")
	}
}

func TestFollowRedirectChain_PermanentRedirect308(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/final")
		w.WriteHeader(http.StatusPermanentRedirect)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	chain, err := FollowRedirectChain(context.Background(), srv.URL+"/start")
	if err != nil {
		t.Fatalf("FollowRedirectChain: %v", err)
	}
	if !chain.Permanent {
		t.Error("Permanent = false, want true for 308 chain")
	}
}

func TestFollowRedirectChain_TemporaryMakesChainNonPermanent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/middle", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/final")
		w.WriteHeader(http.StatusMovedPermanently) // 301
	})
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/middle")
		w.WriteHeader(http.StatusFound) // 302 — poisons chain
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	chain, err := FollowRedirectChain(context.Background(), srv.URL+"/start")
	if err != nil {
		t.Fatalf("FollowRedirectChain: %v", err)
	}
	if chain.FinalURL != srv.URL+"/final" {
		t.Errorf("FinalURL = %q, want %q", chain.FinalURL, srv.URL+"/final")
	}
	if chain.Permanent {
		t.Error("Permanent = true, want false when chain has a 302")
	}
}

func TestFollowRedirectChain_Loop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/b")
		w.WriteHeader(http.StatusMovedPermanently)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/a")
		w.WriteHeader(http.StatusMovedPermanently)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := FollowRedirectChain(context.Background(), srv.URL+"/a")
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") && !strings.Contains(err.Error(), "loop") {
		t.Errorf("error should mention cycle/loop; got %v", err)
	}
}

func TestFollowRedirectChain_ExceedsMaxHops(t *testing.T) {
	// Create a chain longer than maxRedirectHops by chaining /0 → /1 → ...
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Every URL redirects to itself + "x" up to a long chain. Path "/"
		// handles everything else implicitly.
		next := r.URL.Path + "x"
		w.Header().Set("Location", next)
		w.WriteHeader(http.StatusMovedPermanently)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, err := FollowRedirectChain(context.Background(), srv.URL+"/start")
	if err == nil {
		t.Fatal("expected error for exceeding max hops")
	}
	if !strings.Contains(err.Error(), "exceeded") && !strings.Contains(err.Error(), "hops") {
		t.Errorf("error should mention hops/exceeded; got %v", err)
	}
}

func TestFollowRedirectChain_RelativeLocation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/new", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/docs/old", func(w http.ResponseWriter, r *http.Request) {
		// Relative Location — must resolve against currentURL.
		w.Header().Set("Location", "new")
		w.WriteHeader(http.StatusMovedPermanently)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	chain, err := FollowRedirectChain(context.Background(), srv.URL+"/docs/old")
	if err != nil {
		t.Fatalf("FollowRedirectChain: %v", err)
	}
	if chain.FinalURL != srv.URL+"/docs/new" {
		t.Errorf("FinalURL = %q, want %q", chain.FinalURL, srv.URL+"/docs/new")
	}
}

func TestFollowRedirectChain_MissingLocationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMovedPermanently) // no Location header
	}))
	defer srv.Close()

	_, err := FollowRedirectChain(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for missing Location header")
	}
}

func TestFollowRedirectChain_404TerminatesAsFinal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// 404 is a non-redirect status — the chain terminates, but callers will
	// treat the FinalURL as the failed target since no redirect resolved.
	chain, err := FollowRedirectChain(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FollowRedirectChain: %v", err)
	}
	if chain.FinalURL != srv.URL {
		t.Errorf("FinalURL = %q, want %q", chain.FinalURL, srv.URL)
	}
}

func TestResolveRedirectTarget(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		location string
		want     string
	}{
		{"absolute", "https://a.com/foo", "https://b.com/bar", "https://b.com/bar"},
		{"absolute path", "https://a.com/docs/old", "/docs/new", "https://a.com/docs/new"},
		{"relative path", "https://a.com/docs/old", "new", "https://a.com/docs/new"},
		{"query only", "https://a.com/foo?x=1", "?y=2", "https://a.com/foo?y=2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveRedirectTarget(tt.current, tt.location)
			if err != nil {
				t.Fatalf("resolveRedirectTarget: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
