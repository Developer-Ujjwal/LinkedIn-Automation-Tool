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
- **Full Workflow**: Search -> Connect -> Detect Acceptance -> Send Follow-up Message

## Demo

[Watch the Demo Video](https://drive.google.com/file/d/11b0I6hQI6Dl5Y-0vevfxVaO2dNV89YX8/view?usp=sharing)

## Architecture

```
/
├── cmd/bot/main.go          # Entry point
├── config/                  # Configuration (Viper)
├── internal/
│   ├── core/               # Domain types & Interfaces
│   ├── browser/            # Rod wrapper (CDP-based stealth)
│   ├── stealth/            # Humanizer engine (Mouse, Keyboard, Jitter)
│   ├── repository/         # SQLite implementation
│   └── workflows/          # Business Logic (Auth, Search, Connect, Messaging)
├── pkg/utils/              # Helpers (Working hours, cooldowns)
└── data/                   # Cookies, database, & debug dumps
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

### 1. Search & Connect
Search for profiles and send connection requests.
```bash
# Basic usage
./bot.exe -keyword "software engineer"

# With custom note and max results
./bot.exe -keyword "data scientist" -max 20 -note "Hi {{Name}}, I'd like to connect!"

# With location filter
./bot.exe -keyword "developer" -location "New York" -max 15
```

### 2. Manage Connections & Follow-ups
Detect who accepted your requests and send them a welcome message.
```bash
# Step 1: Scan for new connections (updates DB status from 'RequestSent' to 'Connected')
./bot.exe -scan

# Step 2: Send follow-up messages to new connections (limit 5 per run)
./bot.exe -followup

# Combined workflow: Scan, Follow-up, then Search & Connect
./bot.exe -scan -followup -keyword "product manager"
```

### Command Line Flags

- `-config`: Path to config file (default: `config/config.yaml`)
- `-keyword`: Search keyword (required for search mode)
- `-max`: Maximum profiles to connect with (default: 10)
- `-location`: Location filter (optional)
- `-note`: Connection note template with `{{Name}}` placeholder
- `-scan`: Scan "My Network" for new connections
- `-followup`: Send follow-up messages to pending connections

## Features

### 🤖 Stealth & Humanization
- **Advanced Mouse Engine**: Physics-based Bézier curves with acceleration/deceleration (Fitts's Law).
- **CDP Input Events**: Uses Chrome DevTools Protocol for "trusted" input events (bypasses JS detection).
- **Humanized Typing**: Variable WPM, typos with auto-correction, and natural delays.
- **Randomized Timing**: Jitter added to all actions; never sleeps for exact integers.

### 🔄 Automation Workflows
- **Smart Connection**: 
  - Detects "Connect" vs "Message" buttons.
  - Handles "More" dropdowns and "Add a note" modals.
  - Auto-dumps HTML on failure for debugging.
- **Connection Tracking**: 
  - Scans "Recently Added" to detect accepted requests.
  - Updates local database state automatically.
- **Follow-up System**: 
  - Sends personalized welcome messages to new connections.
  - Prevents duplicate messages via database tracking.

### 🛡️ Safety & Limits
- **Session Persistence**: Cookie-based authentication (avoids repeated logins).
- **Rate Limiting**: Daily action limits (Connects/Messages) tracked in SQLite.
- **Working Hours**: Configurable time windows (e.g., 9 AM - 5 PM).
- **Cooldowns**: Random delays between actions (2-8 minutes).
- **Duplicate Prevention**: Database tracking of processed profiles.

## Configuration

Edit `config/config.yaml` to customize:
- Stealth parameters (typing speed, mouse behavior, scrolling)
- Rate limits and working hours
- CSS selectors (for LinkedIn UI changes)
- Message templates
- Database and session paths

## Data Storage

- **Database**: `data/bot.db` (SQLite) - Stores profiles and history
- **Cookies**: `data/cookies.json` - Session persistence


## License

Educational use only.

