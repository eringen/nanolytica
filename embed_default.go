//go:build !embed
// +build !embed

package main

import "github.com/labstack/echo/v4"

func serveEmbeddedStatic(e *echo.Echo) {
	// Default: serve from filesystem
	e.Static("/static", "static")
}
