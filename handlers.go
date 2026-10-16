package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func handleInventory(w http.ResponseWriter, r *http.Request) {
	ecuBins, tcuBins, awdBins := DiscoverBinaries()
	dbs := DiscoverDatabases()
	surfaces := GetSurfacePresets()
	presets := GetScenarioPresets(ecuBins, tcuBins, awdBins)

	resp := InventoryResponse{
		ECUBins:   ecuBins,
		TCUBins:   tcuBins,
		AWDBins:   awdBins,
		Databases: dbs,
		Surfaces:  surfaces,
		Presets:   presets,
	}
	writeJSON(w, http.StatusOK, resp)
}

func handlePlatformSpec(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, GetStingerG70PlatformSpec())
}

func handleCanMatrix(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, GetCCANMatrixCatalog())
}

func handleSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req SimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}
	resp, err := ExecuteSimulation(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleQueryMaps(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	module := q.Get("module")
	if module == "" {
		module = "ecu"
	}
	search := q.Get("search")
	category := q.Get("category")
	limit, _ := strconv.Atoi(q.Get("limit"))

	maps, err := QueryDatabaseMaps(module, search, category, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, maps)
}

func handleCompareMap(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	module := q.Get("module")
	addrHex := q.Get("addr")
	binA := q.Get("bin_a")
	binB := q.Get("bin_b")

	if module == "" || addrHex == "" {
		writeError(w, http.StatusBadRequest, "module and addr parameters are required")
		return
	}

	res, err := CompareMapAcrossBins(module, addrHex, binA, binB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func handleExportCanDump(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req SimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}
	resp, err := ExecuteSimulation(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var buf bytes.Buffer
	buf.WriteString("# Kia Stinger GT / Genesis G70 3.3T 8AT AWD — Full 9-ID C-CAN Emulation Trace (Simulation Master)\n")
	buf.WriteString("# Arbitration IDs: 0x080 (ESP12), 0x153 (TCS11), 0x162 (TCU11), 0x164 (TCU12), 0x220 (AWD11), 0x316 (EMS11), 0x329 (EMS12), 0x386 (WHL_SPD11), 0x4A0 (CLU11)\n")
	buf.WriteString(fmt.Sprintf("# Run: %s | ECU: %s | TCU: %s | AWD: %s\n",
		resp.RunA.Label, resp.RunA.Calibration.ECUBinName, resp.RunA.Calibration.TCUBinName, resp.RunA.Calibration.AWDBinName))

	for _, s := range resp.RunA.Telemetry {
		frames := EncodeTelemetryToCCANFrames(s, resp.RunA.Calibration, resp.RunA.Config.DriveMode)
		for _, f := range frames {
			buf.WriteString(f.CandumpLine + "\n")
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"simulation_candump.log\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
