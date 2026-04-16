# OmniInspect - Deployment Guide

**Date:** 2026-04-15

## Scope

OmniInspect is distributed as a compiled desktop or terminal binary rather than a hosted web service. Deployment in this repository primarily means building and packaging the executable with the required Oracle client dependencies and shipped assets.

## Build Outputs

- Main binary: `omniview` on Unix-like systems, `omniview.exe` on Windows
- ODPI artifact: platform-specific output under `third_party/odpi/lib`
- Runtime assets: Oracle SQL and init assets under `assets/`, images under `resources/`

## Release-Oriented Build Notes

The Makefile distinguishes local development from release builds through CGO linker settings.

### Local builds

- Use system Oracle Instant Client paths
- Prefer `make build` and `make run`

### Release builds

- Use `RELEASE=1` so rpath behavior targets packaged runtime locations
- Ensure packaged binaries can resolve:
  - the application binary
  - `third_party/odpi/lib`
  - the bundled Oracle Instant Client path expected by the target environment

## Platform Considerations

### macOS ARM64

- Default Instant Client location: `/opt/oracle/instantclient_23_7`
- Release rpaths are configured relative to the executable path

### Windows

- Default Instant Client location: `C:\oracle_inst\instantclient_23_7`
- Windows-specific icon embedding is handled through the `icon` target and `rsrc`

## CI and Automation Signals

The repository contains GitHub workflow configuration under `.github/workflows`. This is the primary automation entry point visible in the repo.

## Packaging Considerations

A usable distribution needs:

- The compiled OmniInspect binary
- ODPI shared libraries as required by the target platform
- Oracle Instant Client availability for the target environment
- SQL and init assets required for database package deployment flows
- User-facing resources where platform packaging expects them

## Operational Constraints

- Do not introduce alternate unsupported build paths that bypass the Makefile.
- Do not assume deployment means containerization; this repo is packaging a local executable.
- Treat Oracle client resolution and CGO linking as part of the deployment contract.

## Recommended Release Checklist

1. Verify environment prerequisites on the target platform.
2. Build with the Makefile and appropriate release settings.
3. Confirm the executable resolves Oracle and ODPI shared libraries.
4. Smoke-test onboarding, database connection, and trace listener startup.
5. Confirm bundled assets are present and accessible.

---

_Generated using BMAD Method `document-project` workflow_
