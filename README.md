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
│   └── workflows/          # Business Logic (Auth, Search, Connect)
├── pkg/utils/              # Helpers (Working hours, cooldowns)
└── data/                   # Cookies & database
```

## Setup

1. **Install dependencies:**
```bash
go mod download
```

2. **Configure:**
   - Edit `config/config.yaml` and set your credentials
   - Or set environment variables:
     - `LINKEDIN_BOT_EMAIL`
     - `LINKEDIN_BOT_PASSWORD`

3. **Build:**
```bash
go build -o bot.exe cmd/bot/main.go
```

## Usage

```bash
# Basic usage (required: -keyword)
./bot.exe -keyword "software engineer"

# With custom note and max results
./bot.exe -keyword "data scientist" -max 20 -note "Hi {{Name}}, I'd like to connect!"

# With location filter
./bot.exe -keyword "developer" -location "New York" -max 15

# Custom config file
./bot.exe -config custom-config.yaml -keyword "designer"
```

### Command Line Flags

- `-config`: Path to config file (default: `config/config.yaml`)
- `-keyword`: Search keyword (required)
- `-max`: Maximum profiles to connect with (default: 10)
- `-location`: Location filter (optional)
- `-note`: Connection note template with `{{Name}}` placeholder (default: "Hi {{Name}}, I'd like to connect with you.")

## Features

### Stealth & Humanization
- **Bézier Curve Mouse Movements**: Natural mouse paths with overshoot correction
- **Humanized Typing**: Variable WPM, typos with auto-correction
- **Randomized Timing**: Never exact integer delays
- **Human Scrolling**: Acceleration/deceleration with pauses

### Automation Features
- **Session Persistence**: Cookie-based authentication
- **2FA Support**: Manual intervention handling
- **Rate Limiting**: Daily action limits with database tracking
- **Working Hours**: Configurable time windows
- **Cooldowns**: Random delays between connections (3-8 minutes)
- **Duplicate Prevention**: Database tracking of processed profiles

### Architecture
- **Clean Architecture**: Separation of concerns with interfaces
- **Dependency Injection**: Testable, modular design
- **Structured Logging**: Zap logger for debugging
- **Error Handling**: Graceful degradation

## Configuration

Edit `config/config.yaml` to customize:
- Stealth parameters (typing speed, mouse behavior, scrolling)
- Rate limits and working hours
- CSS selectors (for LinkedIn UI changes)
- Database and session paths

## Data Storage

- **Database**: `data/bot.db` (SQLite) - Stores profiles and history
- **Cookies**: `data/cookies.json` - Session persistence


## License

Educational use only.

