# datastar-template

A minimal starter for [Datastar](https://data-star.dev) apps in Go: [chi](https://github.com/go-chi/chi) router, [templ](https://templ.guide) views, self-hosted fonts, Bootstrap 5.

## Stack

- **Router:** chi
- **Views:** templ (`.templ` -> generated `_templ.go`)
- **Frontend:** Datastar (hypermedia/SSE, client lib in `web/static/js`), Bootstrap 5
- **Static assets:** embedded in the binary, served content-hashed (`hashfs`) for aggressive caching
- **Dev loop:** `air` (rebuild + reload on save)
- **Task runner:** [Task](https://taskfile.dev) (`Taskfile.yml`)

## Running

```sh
task setup      # install pinned tool deps (air, templ) and tidy go.mod
task            # run the dev server with hot reload (default task)
```
Server listens on `:59876` by default (override with `PORT` in `.env` or the environment).

## File naming convention

Inside `web/`:

- `p_*.templ` — a **page**: a full route's top-level template (e.g. `p_home.templ`, `p_404.templ`)
- `pc_*.templ` — a **page component**: shared/reusable pieces pages are built from (e.g. `pc_layout.templ`)

Generated `*_templ.go` files are build artifacts — don't edit them, don't commit changes to them by hand, run `task templ:build` (or let `air` do it).

## Building for production

```sh
task go:build   # generates templ files, builds ./release/app
```

## Adding a page

1. Add `web/p_<name>.templ` using `@Layout(title, Version) { ... }`.
2. Register its route in `web/httpServer.go` (`setupRoutes`).
3. Reference any new static assets via `StaticPath("css/foo.css")` / `StaticPath("js/foo.js")` — never link static files directly, so caching stays correct.

## Branches

**nats-sqlite** branch shows you how to extend this project with [NATS](https://github.com/nats-io/nats.go) and [sqlite3](https://www.sqlite.org/lang.html)
