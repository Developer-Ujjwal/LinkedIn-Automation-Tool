# LinkedIn Automation Tool - Technical POC

**⚠️ DISCLAIMER: This project is for EDUCATIONAL AND TECHNICAL EVALUATION PURPOSES ONLY.**
It is a local assignment to demonstrate proficiency in Go, software architecture, and anti-detection algorithms.
**NEVER use this on production accounts.**

## Overview

A CLI-based automation tool built in Go using `go-rod/rod` that demonstrates:
- Browser automation with stealth features
- Humanized interactions (Bézier mouse movements, variable typing speeds)
- Session persistence and rate limiting
- Clean architecture with dependency injection

## Architecture

```
/
├── cmd/bot/main.go          # Entry point
├── config/                  # Configuration (Viper)
├── internal/
│   ├── core/               # Domain types & Interfaces
│   ├── browser/            # Rod wrapper
│   ├── stealth/            # Humanizer engine
│   ├── repository/         # SQLite implementation
│   └── workflows/          # Business Logic
├── pkg/utils/              # Helpers
└── data/                   # Cookies & database
```

## Setup

1. **Install dependencies:**
```bash
go mod download
```

2. **Configure:**
   - Copy `config/config.yaml` and set your credentials
   - Or set environment variables:
     - `LINKEDIN_BOT_EMAIL`
     - `LINKEDIN_BOT_PASSWORD`

3. **Build:**
```bash
go build -o bot cmd/bot/main.go
```

## Status

🚧 **In Progress** - Step 1 Complete (Core & Config)

## License

Educational use only.

