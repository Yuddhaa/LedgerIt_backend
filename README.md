# LedgerIt_backend
A simple ledger to maintain expense or income by the employees.

---

## 📂 Project Structure

```plaintext

~/Yuddhaa/LedgerIt_backend/..
  cmd/main
  └  main.go
   internal
     auth
     db
     helpers
     server  # this is the important package which co-ordinates all other internal packages and is called by main.go
   sql
     migrations  # used by goose for easy updation to tables and to maintain migration versions 
     queries    # both of these are used by sqlc to generate internal/db
     schema
   .air.toml     # for hot reloading
   .env
   .gitignore
   Makefile
  󰂺 README.md
   go.mod
   go.sum
  󰌱 log.log
   sqlc.yaml    # sqlc configs


ps: this will keep getting updated.
```
---

## ⚙️ Setup Guide

### 1️⃣ Install Go
Download and install Go from the [official website](https://go.dev/dl/).

Verify installation:
```bash
go version
````

---

### 2️⃣ Set up PostgreSql

* Install postgres locally for now.
* Create a new database named:

```
ledgerit
```
---

### 3️⃣ Clone the Repository

```bash
git clone https://github.com/Yuddhaa/LedgerIt_backend.git 
cd LedgerIt_backend
```

---

### 4️⃣ Download Go Modules

```bash
go mod download
```

#### To set up the tabes in database:
```bash
cd ./sql/migrations/
goose postres postgres://<username>:@localhost:5432/ledgerit up
```
* if goose is not installed, run:
```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose --version
```

---

### 5️⃣ Configure Environment Variables

Create a `.env` file in the project root and add:

```env
PORT=3000
DBURL=postgres://<username>:@localhost:5432/ledgerit
GOOGLE_CLIENT_ID= --- put client id ---
JWT_SECRET=4y1oahywoerhFLDhFlFDHZjdqO+p5LHByj4BYvTGiBAgw3IwuoRILSakghhwPhaJHDKS+7IlTykcyy7LyItuEA==
```

---

### 6️⃣ Run the Application

```bash
make run
```

or, without Makefile:

```bash
go run ./
```

---

### 7️⃣ Verify the Server

Once running, you can check:

```
http://localhost:3000
```

---

## 📜 API Documentation

📄 Full API Reference: [View Postman Docs](https://documenter.getpostman.com/view/47074287/2sB3WquLSZ#c78392de-daca-49a3-9191-1e6be32f9214)

---

## 📌 Notes

* Ensure PostgreSql is running before starting the app.
* Ensure all tables with appropriate columns are created. (Just use goose, it's easy to manage the changes to db, if any)
* Environment variables are required for DB connection.
* Default server port: **3000** (can be changed in main.go or .env file).
