// Package templates menyematkan seluruh berkas template HTML ke dalam binary.
// Berkas ini berada di folder templates karena //go:embed tidak dapat
// menjangkau folder induk; package handler menerima hasilnya sebagai fs.FS.
package templates

import "embed"

// FS berisi semua berkas *.html di folder ini.
//
//go:embed all:layout all:pages all:partials
var FS embed.FS
