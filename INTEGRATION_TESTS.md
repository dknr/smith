# Smith Integration Tests

End-to-end tests for the Smith agent client/server with real LLM provider.

## Prerequisites

- Server running: `./smith serve --log-protocol`
- Client built: `go build .`
- Fresh server (no accumulated history)

## Setup

```bash
kill $(lsof -ti:26856) 2>/dev/null; sleep 1; rm -f smith-protocol.log
./smith serve --log-protocol
sleep 2
```

## Run Tests

```bash
./smith send "<message>" --verbose 2>&1
```

- Yellow `[33m` = tool calls
- Grey `[90m` = stats line
- Red `[31m` = errors

## Test Suite

### Basic Greeting

```bash
./smith send "Hello, world!" --verbose 2>&1
```

**Expected:** Direct text response, no tool calls.

### Lua Tool — Simple Print

```bash
./smith send "Use the lua tool to print 'hello from lua'" --verbose 2>&1
```

**Expected:** `smith.print('hello from lua')` — 1 attempt.

### Lua Tool — Math

```bash
./smith send "Use the lua tool to calculate 17 * 42 and print the result" --verbose 2>&1
```

**Expected:** `smith.print(17 * 42)` → 714 — 1 attempt.

### Lua Tool — Directory Listing

```bash
./smith send "Use the lua tool to list the current directory and print the number of entries" --verbose 2>&1
```

**Expected:** `smith.list(".")` with `#entries` — 1 attempt.

### Lua Tool — Sieve of Eratosthenes

```bash
./smith send "Use the lua tool to implement the Sieve of Eratosthenes to find all primes from 0 to 100, then print them as a 10x4 grid (10 rows, 4 columns, comma-separated). Use smith.print for output." --verbose 2>&1
```

**Expected:** 25 primes in 10×4 grid. May loop 3–4 times (padding/formatting).

### Lua Tool — Fibonacci Grid

```bash
./smith send "Use the lua tool to compute the first 50 Fibonacci numbers and print them as a 5-row by 10-column grid, comma-separated." --verbose 2>&1
```

**Expected:** 50 Fibonacci numbers in 5×10 grid. May loop 2–3 times (padding style).

### Lua Tool — Collatz Sequence

```bash
./smith send "Use the lua tool to compute the Collatz sequence for n=27 (which has the longest sequence for n<30). Print the full sequence and its length." --verbose 2>&1
```

**Expected:** 112 elements. May loop 3–4 times (looping/formatting).

### Lua Tool — Perfect Numbers

```bash
./smith send "Use the lua tool to find all perfect numbers up to 1000. A perfect number equals the sum of its proper divisors. Print each one." --verbose 2>&1
```

**Expected:** 6, 28, 496 — 1 attempt.

### Lua Tool — Pascal's Triangle

```bash
./smith send "Use the lua tool to generate Pascal's triangle with 10 rows. Print each row on its own line, with numbers separated by spaces." --verbose 2>&1
```

**Expected:** 10 rows of Pascal's triangle — 1 attempt.

### Lua Tool — Multiplication Table

```bash
./smith send "Use the lua tool to create a 5x5 multiplication table. Print each row on one line with values separated by tabs (use smith.print with multiple args)." --verbose 2>&1
```

**Expected:** 5×5 table with tab separators. May loop 1–2 times (multi-arg vs single string).

### Lua Tool — Longest Common Subsequence

```bash
./smith send "Use the lua tool to find the longest common subsequence of 'ABCBDAB' and 'BDCAB'. Print the LCS string and its length." --verbose 2>&1
```

**Expected:** LCS = "BCAB", length 4 — 1 attempt.

### Lua Tool — Prime Factorization (small)

```bash
./smith send "Use the lua tool to find the prime factorization of 13195. Print each prime factor and its exponent." --verbose 2>&1
```

**Expected:** 5¹, 7¹, 13¹, 29¹ — 1 attempt.

## Known Failures (Timeouts)

These tests exceed the 60s timeout — avoid for now:

- **Anagram grouping** — LLM loops on hash-map-like keying
- **Prime factorization of large numbers** (>10⁹) — exceeds Lua number precision or infinite loops

## Cleanup

```bash
kill $(lsof -ti:26856) 2>/dev/null; rm -f smith-protocol.log
```
