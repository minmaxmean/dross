# Dross - Personal Assistant

Dross is an AI-powered personal assistant built in Go using Google's **Agent Development Kit (ADK)** and local **Gemma** models executing via **Ollama**.

The project is structured to run in two distinct modes:
1. **CLI Mode**: Interactive terminal-based REPL loop.
2. **Telegram Bot Mode**: A background daemon that communicates with authorized Telegram users, supporting isolated threads as distinct conversation contexts and native in-progress animation via Telegram's `sendMessageDraft` API.

---

## Directory Structure

```
dross/
├── cli/
│   └── cli.go                 # Terminal CLI runner loop
├── telegram/
│   └── telegram.go            # Telegram Bot server, auth check, and drafting loop
├── ollama/
│   └── ollama.go              # Custom ADK model.LLM implementation for Ollama API
├── scripts/
│   └── telegram_test_client.py# Telethon script to test Telegram bot end-to-end
├── main.go                    # App router flag parser and .env loader
├── go.mod                     # Go dependency tree
├── .env                       # Local environment configurations (ignored by Git)
└── .env.example               # Template environment configuration file
```

---

## Setup & Prerequisites

### 1. Ollama
Ensure Ollama is installed on your Mac and the target Gemma model is pulled and running:
```bash
ollama run gemma4:e2b
```
*(By default, the assistant targets `gemma4:e2b` but you can configure different versions in the codebase or environment).*

### 2. Environment Configuration
Create a `.env` file at the root of the project by copying `.env.example`:
```bash
cp .env.example .env
```
Fill out the keys in `.env`:
* `TELEGRAM_BOT_TOKEN`: Token obtained from [@BotFather](https://t.me/botfather).
* `TELEGRAM_ALLOWED_USERS`: Comma-separated list of Telegram User IDs authorized to interact with the bot.
* `TELEGRAM_API_ID`: App ID obtained from [my.telegram.org](https://my.telegram.org).
* `TELEGRAM_API_HASH`: App Hash obtained from [my.telegram.org](https://my.telegram.org).
* `TELEGRAM_TEST_PHONE`: Phone number of your Telegram testing account.

---

## Running the Assistant

### Mode A: Interactive Terminal (CLI)
To run the assistant locally in your terminal console:
```bash
go run main.go -mode=cli
```

### Mode B: Telegram Bot Daemon
To launch the Telegram bot:
```bash
go run main.go -mode=telegram
```
Once started, the bot registers handlers, loads your memory sessions, and waits for incoming updates.

---

## Testing & Verification

### Automated Integration Test (Real Telegram Network)
To test the bot end-to-end without mocks, we use a headless user client script written in Python using `Telethon`.

1. Ensure the Python dependency is installed:
   ```bash
   pip install telethon
   ```
2. Start your bot in Telegram mode in one terminal:
   ```bash
   go run main.go -mode=telegram
   ```
3. Run the Python verification script in another terminal:
   ```bash
   python3 scripts/telegram_test_client.py --bot your_bot_username --message "Explain recursion in one sentence"
   ```
4. **First Run Only**: The script will prompt you for the verification login code sent to your test account. Type it into the terminal. Once logged in, a local `.session` file is saved, enabling fully automated, passwordless test runs in the future.