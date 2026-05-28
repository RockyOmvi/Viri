# Viri Testnet User Guide

## Overview

Viri Testnet is a Layer 1 blockchain running HotStuff-2 BFT consensus with native EVM support. This guide covers how to connect your wallet, claim testnet tokens, and interact with deployed smart contracts.

## Quick Start

1. Add Viri Testnet to MetaMask
2. Claim tokens from the faucet
3. Import the VIRI Token (ERC-20)
4. Start building

---

## 1. Adding Viri Testnet to MetaMask

### Automatic (Recommended)

Visit [https://faucet.viri.me](https://faucet.viri.me) and click "Add to MetaMask" (future feature).

### Manual Configuration

Open MetaMask → Settings → Networks → Add Network:

| Field | Value |
|---|---|
| Network Name | Viri Testnet |
| RPC URL | `https://rpc.viri.me` |
| Chain ID | `1987050601` (`0x76697269`) |
| Currency Symbol | `VIRI` |
| Block Explorer | `https://testnet.viri.me` |

## 2. Claiming Testnet Tokens

Visit [https://faucet.viri.me](https://faucet.viri.me):

1. Enter your wallet address (0x...)
2. Select amount multiplier (1x, 5x, or 10x)
3. Complete the captcha
4. Click "Request VIRI Tokens"

Each claim distributes both:
- **Native VIRI** — used for gas fees
- **VIRI Token (ERC-20)** — standard ERC-20 token

Cooldown: 10 seconds between claims
Daily limit: 100 VIRI (native) per address

## 3. Importing the VIRI Token (ERC-20)

In MetaMask, go to **Tokens** → **Import Tokens** and enter:

- **Token Contract Address**: `0x00000000000000000000000000000000000000E0`
- **Token Symbol**: `VIRI`
- **Decimals**: `18`

## 4. Importing the VIRI NFT (ERC-721)

To view your NFTs in MetaMask, go to **NFTs** → **Import NFTs** and enter:

- **NFT Contract Address**: `0x00000000000000000000000000000000000000E1`
- **Token ID**: `1` (or 2, 3 for genesis tokens)

## 5. Contract Addresses

| Contract | Address | Notes |
|---|---|---|
| VIRI Token (ERC-20) | `0x00000000000000000000000000000000000000E0` | 1,000,000 supply, 18 decimals |
| VIRI NFT (ERC-721) | `0x00000000000000000000000000000000000000E1` | 3 genesis tokens minted |
| Faucet | `0x3bb1676433a48e338d0b7c632fde91e8d8345b07` | Funds new accounts |

Standard contracts are built into the protocol (native precompiles) — always available at the same addresses across all Viri networks.

## 6. Network Endpoints

| Service | URL |
|---|---|
| RPC (JSON-RPC) | `https://rpc.viri.me` |
| Explorer | `https://testnet.viri.me` |
| Faucet | `https://faucet.viri.me` |

## 7. Sending Transactions

Use any EVM-compatible wallet or tool:

```bash
# Using curl
curl -X POST https://rpc.viri.me \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "method":"eth_sendRawTransaction",
    "params":["0x..."],
    "id":1
  }'
```

### Gas Parameters

- **Gas Price**: 1 Gwei (fixed)
- **Gas Limit**: 200,000 for typical transactions
- **Block Time**: ~500ms
- **Finality**: Instant (1 block)

## 8. ERC-20 Token Functions

Callable via `eth_call` (read-only) or `eth_sendRawTransaction` (state-changing):

| Function | Selector | Description |
|---|---|---|
| `name()` | `0x06fdde03` | Returns "VIRI Token" |
| `symbol()` | `0x95d89b41` | Returns "VIRI" |
| `decimals()` | `0x313ce567` | Returns 18 |
| `totalSupply()` | `0x18160ddd` | Returns 1,000,000 |
| `balanceOf(address)` | `0x70a08231` | Token balance of address |
| `transfer(to, amount)` | `0xa9059cbb` | Transfer tokens |
| `mint(to, amount)` | `0x40c10f19` | Mint new tokens (testnet only) |

## 9. ERC-721 NFT Functions

| Function | Selector | Description |
|---|---|---|
| `name()` | `0x06fdde03` | Returns "VIRI NFT" |
| `symbol()` | `0x95d89b41` | Returns "VNFT" |
| `totalSupply()` | `0x18160ddd` | Returns 3 |
| `ownerOf(tokenId)` | `0x6352211e` | Owner of token ID |
| `tokenURI(tokenId)` | `0xc87b56dd` | Metadata URI for token |
| `mint(to, tokenId)` | `0xa0710e8d` | Mint new NFT (testnet only) |

## 10. Example: Check Token Balance via cURL

```bash
# ABI-encode balanceOf(faucet_address)
curl -X POST https://rpc.viri.me \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_call",
    "params": [{
      "to": "0x00000000000000000000000000000000000000E0",
      "data": "0x70a082310000000000000000000000003bb1676433a48e338d0b7c632fde91e8d8345b07"
    }, "latest"],
    "id": 1
  }'
```

Decode the hex result:
- `0x0000...0000d3c21bcecceda1000000` = 1,000,000 × 10^18 (full supply)

## 11. Features

- **HotStuff-2 BFT**: ~500ms block time, instant finality
- **EVM Compatibility**: Deploy any Solidity contract
- **Native ERC-20/ERC-721**: Standard contracts built into protocol
- **Account Abstraction**: Social recovery, session keys, gas sponsorship
- **Privacy**: ZK shielded transfers (experimental)
- **Cross-Chain**: IBC and bridge support
- **Governance**: On-chain DAO voting
