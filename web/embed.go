package web

import (
	"embed"
	"io/fs"
)

//go:embed *.html static/*
var Assets embed.FS

// StaticFS is the sub-filesystem for serving static CSS and JS assets
var StaticFS, _ = fs.Sub(Assets, "static")
