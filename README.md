# Snap

> State-aware checkpoints for AI-agent workflows.

When you (or an AI agent) rewind your git history to an older commit, your codebase changes, but your external state (databases, `.env` files, local caches) stays exactly where it was. This mismatch creates **Agentic Drift** — your code expects one reality, but your system state reflects another.

**Snap** solves this by directly intercepting Git hooks to provide automatic, invisible state synchronization tied directly to your Git commit hash.

---

## 🧠 How it Works

Snap operates on a single truth: **The Git Commit Hash dictates the entire system constraints.**

1. You run `git commit`. 
2. Snap's `post-commit` hook triggers automatically.
3. Snap executes its **Driver Registry** (e.g., SQLite, Dotenv) to capture your system state.
4. Binary state is streamed into a **Content-Addressable Store (CAS)** using bounded memory buffers.
5. You run `git checkout HEAD~1`.
6. Snap's `post-checkout` hook intercepts the checkout.
7. Snap instantly queries the CAS and overwrites your databases and configs with the exact binary blobs associated with that older commit.

---

## 🛠 Installation

```bash
# Install the CLI directly to your Go bin
go install github.com/NishthaNabya/snap/cmd/snap@latest
```

---

## 🚀 Quick Start

### 1. Initialization
Navigate to any git repository and initialize Snap. This creates the hidden `.snap/` storage engine and safely installs chaining Git hooks.

```bash
snap init
```

### 2. Configuration
Tell Snap what state you want to bind to your git history by creating `.snap/config.json`. We use a **Priority Loading** architecture, meaning Environment drivers will always restore *before* Database drivers.

```json
{
  "entries": [
    {
      "driver": "dotenv",
      "source": ".env"
    },
    {
      "driver": "sqlite",
      "source": "local.db"
    }
  ]
}
```

### 3. See the Magic
From here on out, Snap is invisible. Just use Git.

```bash
# Make some State Changes
echo "SECRET_KEY=123" > .env
sqlite3 local.db "CREATE TABLE users (id int);"

# A standard commit (Snap captures the state silently in the background!)
git add .env local.db
git commit -m "Added users table and secret key"

# Nuke the database and change the env
echo "SECRET_KEY=ABC" > .env
sqlite3 local.db "DROP TABLE users;"
git commit -am "Destroyed everything"

# Rewind Git. Snap intercepts and restores your DB and .env to the previous state.
git checkout HEAD~1
```

---

## ⚙️ Core Architecture Details

* **Zero-Cost Deduplication:** Blobs are stored in `.snap/objects/` and named by their SHA-256 digest. If your database hasn't changed between commits, Snap stores 0 new bytes.
* **Atomic Writes:** All filesystem mutations follow a strict `TempWrite -> fsync -> Rename` POSIX pipeline. It is impossible to achieve a partially corrupted state during a crash or power loss.
* **Bounded I/O:** Snap uses `io.TeeReader` and 32KB buffer limits for all hashing and streaming. It can capture and restore 50GB database blobs while consuming virtually 0MB of RAM.
* **Idempotent Restoration:** If a restore fails midway entirely (e.g., you `kill -9` the process), you simply run `snap restore <hash>` again. 

---
*Built to eliminate friction in Agentic Coding.*
