# Finality

**Finality** is a lightweight, tamper-proof blockchain written in Go, designed to verify and secure financial transactions. It aims to prevent centralized database manipulation by offering an immutable, auditable ledger.

---

## Features

- Custom blockchain implementation in Go
- Transaction integrity via hashing
- Simple, extendable block and transaction model
- Optional HTTP API for interacting with the node
- Graceful shutdown with SIGINT handling
- Future-proof structure for P2P, consensus, and storage

---

## Getting Started

### 1. Clone the repository

```bash
git clone https://github.com/sammyklan3/finality.git
cd finality
```

### 2. Initialize Go modules

```bash
go mod tidy
```

### 3. Run the node

```bash
go run ./blockchain
```
