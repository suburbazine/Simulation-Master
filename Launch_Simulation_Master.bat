@echo off
title Simulation Master — Tri-ECU Powertrain & Longitudinal Physics Studio
cd /d "%~dp0"
echo Starting Simulation Master on http://localhost:8085 ...
start "" "http://localhost:8085"
simulation_master.exe -port 8085
pause
