# Simulation Master — Local Storage Directory Tree

This directory (`./storage`) is the canonical local storage location for **BinGo SQLite Teardown Databases (`.db`)** and **ECU / TCU / AWD Calibration Binaries (`.bin`)**.

All `.db` and `.bin` files inside this tree are excluded from Git via `.gitignore` so proprietary or large binary dumps are never committed to the repository.

## Folder Hierarchy

```text
storage/
├── README.md
├── db/
│   ├── ecu/        # SIM2K-260 (TC1791) SQLite DB (e.g. flash_analysis.db)
│   ├── tcu/        # Kefico CPTSH 8AT (SH72549R) SQLite DB (e.g. tcu_cptsh_95441_4J571.db)
│   └── awd/        # Magna ATC AWD (95447-4J105) SQLite DB (e.g. awd_95447_4J105.db)
└── bins/
    ├── ecu/        # SIM2K-260 4 MB (4,194,304 B) Full Flash Binaries (.bin)
    ├── tcu/        # Kefico CPTSH 8AT 768 KB / 767 KB Calibration Binaries (.bin)
    └── awd/        # Magna ATC AWD 48 KB / 32 KB Calibration Binaries (.bin)
```

## Importing Files

You can populate this directory in two ways:
1. **Browser File Picker (Recommended)**: Click **`📂 IMPORT DB / BIN`** in the top toolbar of the Simulation Master Web Studio (or click **`+ Import .BIN`** next to any ECU/TCU/AWD selector in the left sidebar) to select or drag-and-drop `.db` and `.bin` files directly from your browser.
2. **Direct File Copy**: Copy `.db` or `.bin` files directly into the corresponding subfolder above and click **Run A/B Emulation** (or refresh).
