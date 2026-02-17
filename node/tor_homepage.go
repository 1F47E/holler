package node

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// HomepageData is passed to the HTML template.
type HomepageData struct {
	Name      string
	Bio       string
	OnionAddr string
	Version   string
}

// Profile is the optional ~/.holler/profile.json format.
type Profile struct {
	Name string `json:"name"`
	Bio  string `json:"bio"`
}

// LoadProfile reads profile.json from the holler directory.
// Returns sensible defaults if the file doesn't exist or is malformed.
func LoadProfile(hollerDir string) Profile {
	path := filepath.Join(hollerDir, "profile.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{Name: "holler agent"}
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{Name: "holler agent"}
	}
	if p.Name == "" {
		p.Name = "holler agent"
	}
	return p
}

var homepageTmpl = template.Must(template.New("homepage").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}}</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#0d1117;color:#c9d1d9;font-family:monospace;padding:2rem;max-width:600px;margin:0 auto}
h1{color:#58a6ff;margin-bottom:.5rem;font-size:1.4rem}
.bio{color:#8b949e;margin-bottom:1.5rem}
.field{margin-bottom:.75rem}
.label{color:#8b949e;font-size:.85rem}
.value{color:#c9d1d9;word-break:break-all}
.onion{color:#f0883e}
hr{border:none;border-top:1px solid #21262d;margin:1.5rem 0}
.footer{color:#484f58;font-size:.8rem}
</style>
</head>
<body>
<h1>{{.Name}}</h1>
{{if .Bio}}<p class="bio">{{.Bio}}</p>{{end}}
<hr>
<div class="field">
<div class="label">onion</div>
<div class="value onion">{{.OnionAddr}}.onion</div>
</div>
<div class="field">
<div class="label">protocol port</div>
<div class="value">9000</div>
</div>
<hr>
<div class="footer">holler v{{.Version}}</div>
</body>
</html>`))

// StartHomepage serves the agent profile page on the given listener.
// Blocks until ctx is cancelled.
func StartHomepage(ctx context.Context, ln net.Listener, data HomepageData) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if err := homepageTmpl.Execute(w, data); err != nil {
			fmt.Fprintf(os.Stderr, "tor homepage: template: %v\n", err)
		}
	})

	srv := &http.Server{
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 4096,
	}

	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "tor homepage: %v\n", err)
	}
}
