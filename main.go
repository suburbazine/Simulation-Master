package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	port := flag.Int("port", 8085, "HTTP server port for Simulation Master")
	flag.Parse()

	_ = EnsureStorageTree()
	ecuBins, tcuBins, awdBins := DiscoverBinaries()
	dbs := DiscoverDatabases()

	fmt.Println("=======================================================================")
	fmt.Println("  SIMULATION MASTER — Tri-ECU Powertrain & Longitudinal Physics Engine")
	fmt.Println("  Modules: SIM2K-260 (TC1791) | Kefico CPTSH 8AT | Magna ATC AWD")
	fmt.Println("=======================================================================")
	fmt.Printf("  Local Storage Root  : %s\n", GetStorageRoot())
	fmt.Printf("  Discovered Binaries : %d ECU | %d TCU | %d AWD\n", len(ecuBins), len(tcuBins), len(awdBins))
	for _, db := range dbs {
		status := "MISSING"
		if db.Available {
			status = fmt.Sprintf("MOUNTED (%d maps, %d funcs, %d xrefs)", db.MapCount, db.FuncCount, db.XrefCount)
		}
		fmt.Printf("  Database [%s]      : %s\n", db.Module, status)
	}
	fmt.Printf("  Web Studio URL      : http://localhost:%d\n", *port)
	fmt.Println("=======================================================================")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/inventory", handleInventory)
	mux.HandleFunc("/api/simulate", handleSimulate)
	mux.HandleFunc("/api/maps", handleQueryMaps)
	mux.HandleFunc("/api/maps/compare", handleCompareMap)
	mux.HandleFunc("/api/export/candump", handleExportCanDump)
	mux.HandleFunc("/api/platform-spec", handlePlatformSpec)
	mux.HandleFunc("/api/can-matrix", handleCanMatrix)
	mux.HandleFunc("/api/storage", handleStorageStatus)
	mux.HandleFunc("/api/import", handleImportFiles)

	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to mount embedded static filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	addr := fmt.Sprintf(":%d", *port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
