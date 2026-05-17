# IPMS — Intelligent Profile Monitoring System

> **Final Year Project** | University of Agriculture, Faisalabad  
> **Student:** Muhammad Abdullah Mujahid · Reg# 2022-AG-6620  
> **Program:** BS IT · Session 2022–26 · Semester 7  
> **Supervisor:** Department of Information Technology, UAF

---

## Overview

IPMS is a Go-based web application that monitors social media profiles across **X (Twitter)** and **Instagram**. It continuously tracks followers, unfollows, and new activities, then uses **Grok AI (by xAI)** to analyze user behaviour and generate summarized threat intelligence reports. All data is stored securely in a local SQLite database.

---

## Features

| Feature | Description |
|---|---|
| **Public Profile Monitoring** | Track any public X or Instagram profile |
| **Live Scraping** | Real-time data via X API v2 and Instagram oEmbed |
| **Manual Entry** | Add profiles manually when APIs are unavailable |
| **LLM Behaviour Analysis** | Grok AI generates structured threat intelligence reports |
| **Bot Detection** | Rule-based risk scoring (0–100) with configurable thresholds |
| **Risk Classification** | Four tiers: Low / Medium / High / Critical |
| **Web Dashboard** | Dark-themed SPA with live charts and activity feed |
| **Snapshot History** | Daily follower/following snapshots stored in SQLite |
| **WebSocket Alerts** | Real-time push notifications for follower spikes |
| **Auto-refresh Cron** | Every 10 minutes — all monitored profiles refreshed |
| **Secure Storage** | SQLite with WAL mode and foreign key constraints |

---

## Tech Stack

| Layer | Technology |
|---|---|
| **Backend** | Go 1.21 · Gin/Fiber HTTP framework |
| **Database** | SQLite (via mattn/go-sqlite3) |
| **AI Engine** | Grok (xAI) — `grok-3-mini` via OpenAI-compatible API |
| **Real-time** | WebSocket (gorilla/websocket) |
| **Scraping** | X API v2 (Twitter) · Instagram oEmbed · Manual entry |
| **Frontend** | HTML5 · Vanilla JS · Chart.js · CSS Variables |
| **Scheduling** | robfig/cron — auto-refresh every 10 min |

---

## Project Structure

```
IPMS/
├── frontend/
│   └── index.html           ← Full SPA dashboard (open in browser)
│
├── go-backend/
│   ├── cmd/
│   │   └── main.go          ← Entry point, router setup, cron
│   ├── config/
│   │   └── config.go        ← Environment variable loader
│   ├── internal/
│   │   ├── db/
│   │   │   └── db.go        ← SQLite queries, migrations, seeding
│   │   ├── handlers/
│   │   │   └── handlers.go  ← HTTP route handlers (Gin)
│   │   ├── models/
│   │   │   └── models.go    ← Data models / structs
│   │   └── services/
│   │       ├── ai.go        ← Grok AI integration
│   │       ├── hub.go       ← WebSocket broadcast hub
│   │       ├── instagram.go ← Instagram scraper
│   │       ├── risk.go      ← Bot risk scoring engine
│   │       └── twitter.go   ← X (Twitter) API v2 client
│   ├── .env                 ← API keys (never commit this)
│   ├── go.mod
│   └── Makefile
│
└── README.md
```

---

## Prerequisites

- **Go 1.21+** — [golang.org/dl](https://golang.org/dl/)
- **GCC** — required for `go-sqlite3` CGO compilation
  - Windows: install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/)
  - Linux: `sudo apt install gcc`
  - macOS: `xcode-select --install`

---

## Quick Start

### 1. Configure API Keys

Edit `go-backend/.env`:

```env
PORT=5000
DB_PATH=./ipms.db

# Grok AI — free tier at https://console.x.ai
GROK_API_KEY=xai-xxxxxxxxxxxxxxxxxxxx

# X (Twitter) API v2 — free tier at https://developer.twitter.com
X_BEARER_TOKEN=AAAAAAAAAAAAAAAAAAAAAxxxxxxxxxxxx

# RapidAPI (optional) — Instagram scraping fallback
RAPIDAPI_KEY=your_rapidapi_key_here
```

> **Note:** The system works in **offline mode** without API keys.  
> Grok AI will fall back to a built-in rule-based report generator.  
> X API will prompt for manual profile entry.

### 2. Run the Backend

```bash
cd go-backend
make run          # Downloads deps + starts server on :5000
```

Or manually:
```bash
cd go-backend
go mod tidy
go run ./cmd/main.go
```

### 3. Open the Frontend

Simply open `frontend/index.html` in your browser:
```
file:///path/to/IPMS/frontend/index.html
```

Or serve it with any static server:
```bash
cd frontend
python3 -m http.server 3000
# → http://localhost:3000
```

---

## API Reference

Base URL: `http://localhost:5000/api`

### Health
| Method | Endpoint | Description |
|---|---|---|
| GET | `/health` | System status, versions, service states |

### Profiles
| Method | Endpoint | Description |
|---|---|---|
| GET | `/profiles` | List all profiles (supports `?platform=`, `?risk=`, `?search=`) |
| GET | `/profiles/:id` | Get single profile with snapshots and reports |
| PATCH | `/profiles/:id/risk` | Update risk level manually |
| DELETE | `/profiles/:id` | Deactivate (soft delete) a profile |

### Scraping
| Method | Endpoint | Description |
|---|---|---|
| POST | `/scrape/fetch` | Fetch profile data only |
| POST | `/scrape/analyze` | Fetch + run full AI analysis |
| POST | `/scrape/refresh/:id` | Re-scrape an existing profile |
| POST | `/scrape/manual` | Add profile via manual data entry |
| GET | `/scrape/status` | Check API connectivity |

### Analytics
| Method | Endpoint | Description |
|---|---|---|
| GET | `/analytics/stats` | Dashboard summary stats |
| GET | `/analytics/chart` | Daily metrics for charts |
| GET | `/analytics/activity` | Activity log (supports `?severity=`, `?limit=`, `?offset=`) |
| GET | `/analytics/snapshots/:id` | Profile follower history |

### AI Analysis
| Method | Endpoint | Description |
|---|---|---|
| POST | `/analysis/profile/:id` | Run Grok AI analysis on stored profile |
| GET | `/analysis/reports` | List all generated reports |

### WebSocket
| | |
|---|---|
| `ws://localhost:5000/ws` | Real-time event feed |

**WS Event Types:**
- `connected` — welcome message on connect
- `profile_update` — follower count changed
- `alert` — significant follower spike/drop detected

---

## Risk Scoring Engine

The bot-detection algorithm scores profiles 0–100 based on:

| Signal | Max Score | Condition |
|---|---|---|
| Follow/Follower Imbalance | +40 | Following >5000 with ratio <0.1 |
| High Follow Imbalance | +25 | Following >2000 with ratio <0.2 |
| Low Follower Ratio | +15 | Following >500 with ratio <0.5 |
| Empty Bio | +15 | No bio text |
| Very Few Posts | +20 | <5 posts with >100 followers |
| Low Posts / High Following | +15 | <20 posts, following >1000 |
| Very New Account | +20 | Joined 2024 or later |
| Recently Created | +10 | Joined 2023 |
| Spam Keywords in Bio | +10 each | crypto, nft, investment, forex, etc. |
| Not Verified | +5 | No verification badge |

**Risk Levels:** `low` (0–19) · `medium` (20–44) · `high` (45–69) · `critical` (70+)

---

## Grok AI Report Structure

Each generated report contains 7 sections:

1. **Executive Summary** — Overview of profile behaviour
2. **Behaviour Analysis** — Follow ratio, post patterns, account age
3. **Threat Indicators** — Specific flags detected
4. **Bot Probability Score** — 0–100% likelihood of automation
5. **Sentiment Analysis** — Bio and content tone assessment
6. **Recommendations** — Action steps based on risk level
7. **Final Risk Verdict** — Summary classification with justification

---

## SDG Alignment

- **Goal 9** — Industry, Innovation and Infrastructure
- **Goal 16** — Peace, Justice and Strong Institutions

---

## License

Educational project — University of Agriculture, Faisalabad.  
© 2025 Muhammad Abdullah Mujahid. All rights reserved.
