# 🏦 banco

## 📖 Introduction

Banco uses Taproot and Elements introspection opcodes to enable an efficient and decentralized trading protocol between two or more parties using a non-interactive atomic swap construction.

Try it out at [banco.vulpem.com](https://banco.vulpem.com) running a Liquid Testnet instance.

## 🧿 Protocol

### 🍔 TL;DR

It requires two transactions, one to **fund** the "trade contract" and another to either **fulfill** or **cancel** it.
*Anyone* can fulfill the contract as soon is seen on the mempool, but only the maker can cancel it.
The contract enforces that the first output has the requested value, asset and script in the spending transaction.

## Why?

- **Non-interactive** is a desirable property for atomic swaps. It allows the maker to create a transaction that can be fulfilled by the taker without any further interaction from the maker.

- **Easiest integration, ever** The protocol can be used by any wallet that supports sending to an Elements address. No PSBT, no signatures to exchange. Simple!

- **Liquidity efficient**  The maker can create a transaction that can be fulfilled by the taker at any time (as long the maker does not cancel it wih a double-spend). This allows the taker to provide liquidity via automated bots that can fulfill the transaction without the risk of the capital to be locked for a long time (as it happens with time-locked contracts).

- **Decentralized** The protocol is completely decentralized and does not require any trusted third party, being non-custodial for traders and capital efficient for market makers, without the need of centralized "order-book" server. The taker simply observes the mempool for pending contracts to fulfill.

In-depth explanation and comparisons with other trading protocols can be found in the [protocol](./PROTOCOL.md) document.

## 🏃 Show me the code

The most simple way to run Banco locally is using docker. Standalone installation instructions coming soon.

```bash
git clone https://github.com/vulpemventures/banco.git
cd banco
```

### 🌊 Setup Ocean

Ocean is an Elements wallet daemon that is used by Banco, an automated taker bot that fulfills pending contracts in the mempool.

```bash
docker-compose up -d oceand
```

This will run a server on `localhost:18000` and the first time you have to `ocean wallet create`. If you have already a mnemonic you will be able to `ocean wallet restore` it.

Set an alias for the `ocean` command to run it from your terminal:

```bash
alias ocean="docker exec -it oceand ocean"
```

#### Initialize Ocean CLI

```bash
ocean config init --no-tls
```

#### Create or restore a wallet

```bash
ocean wallet create --mnemonic <your_mnemonic> --password <your_password>
# or if your already have a mnemonic
# ocean wallet restore --mnemonic <your_mnemonic> --password <your_password>
```

#### Unlock

```bash
ocean wallet unlock --password <your_password>
```

### 🚚 Run Banco Web Server

This will run a server on `localhost:8080` for the `NETWORK=testnet` and `WATCH_INTERVAL_SECONDS=5` by default.

```bash
docker-compose up -d banco
```

## ⚙️ Environment Variables

Banco uses the following environment variables:

- `WEB_DIR`: The directory where the web files are located. Default is `web`.
- `OCEAN_URL`: The URL of the Ocean node. Default is `localhost:18000`.
- `OCEAN_ACCOUNT_NAME`: The name of the Ocean account. Default is `default`.
- `WATCH_INTERVAL_SECONDS`: The interval in seconds for watching for pending trades to fulfill. Default is `-1`, which means changes are NOT watched continuously.
- `NETWORK`: The network to use. Default is `liquid`.
- `GIN_MODE`: Enable release or debug mode. Default is `debug`.

## 📦 Development

### Requirements

- [Go](https://golang.org/) 1.16 or higher.
- [Ocean Daemon](https://github.com/vulpemventures/oceand) 0.2.0 or higher

### 🏗️ Build Banco

```bash
go mod download
go build -o banco
```

### 🧪 Test

```bash
go test ./...
```

### 📝 Docs

```bash
go doc ./...
```

### Run Web Server

  ```bash
  go run .
  ```

## Contribution 🤝

Pull requests and issues are welcome!

## License 📜

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
