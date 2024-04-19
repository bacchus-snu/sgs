package view

import (
	"embed"
	"io/fs"
)

var (
	//go:embed static/dist/*
	static    embed.FS
	Static, _ = fs.Sub(static, "static/dist")
)
