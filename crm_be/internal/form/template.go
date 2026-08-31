package form

// The go:embed directive can't reach outside the directory its .go file
// lives in, which is why this tiny file sits at internal/form/ (one
// level above template/) rather than inside template/ itself — same
// reasoning migrations/embed.go documents for *.sql.
//
// formPage is parsed with html/template, NEVER text/template — field
// labels come from customer input (Fields.Label) and are rendered into
// HTML; that's the one XSS path in this phase (TD §7), and
// html/template's contextual auto-escaping is what closes it.
// text/template has no such escaping at all.

import (
	"embed"
	"html/template"
)

//go:embed template/form.gohtml
var formPageFS embed.FS

var formPage = template.Must(template.ParseFS(formPageFS, "template/form.gohtml"))

// embedJS is served byte-for-byte at GET /embed.js — a static asset,
// not a template (D8's companion script carries no per-request or
// per-form data at all, see its own file header comment).
//
//go:embed template/embed.js
var embedJS []byte
