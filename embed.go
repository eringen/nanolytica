//go:build embed
// +build embed

package main

import (
	"embed"
	"io/fs"

	"github.com/labstack/echo/v4"
)

//go:embed static/css/* static/js/*
var staticFS embed.FS

func serveEmbeddedStatic(e *echo.Echo) {
	fsys, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	e.StaticFS("/static", fsys)
}
