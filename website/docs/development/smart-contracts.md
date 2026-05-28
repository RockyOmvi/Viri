# Smart Contract Tutorial

Viri is fully EVM-compatible — every standard Ethereum smart contract tool works. This tutorial walks through writing, compiling, deploying, and interacting with a contract on the Viri testnet.

---

## 1. Prerequisites

- **Node.js 18+** and npm
- A funded Viri testnet account (use the [faucet](https://faucet.viri.me))
- Your account's private key (exported from your wallet)

---

## 2. Project Setup

Create a new Hardhat project:

```bash
mkdir viri-token
cd viri-token
npm init -y
npm install --save-dev hardhat @nomicfoundation/hardhat-toolbox ethers
npx hardhat init
```

Select **"Create an empty hardhat.config.js"** when prompted.

Edit `hardhat.config.js` to point at the Viri testnet:

```javascript
require("@nomicfoundation/hardhat-toolbox");

module.exports = {
  solidity: "0.8.28",
  networks: {
    viri: {
      url: "https://rpc.testnet.viri.me",
      chainId: 2,
      accounts: [process.env.PRIVATE_KEY],
    },
  },
};
```

Set your private key as an environment variable:

```bash
export PRIVATE_KEY=0x...
```

> **Security**: Never commit your private key. Use `.env` files or CI secrets.

---

## 3. Write a Contract

Create `contracts/Token.sol`:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Token {
    string public name = "Viri Tutorial Token";
    string public symbol = "VTT";
    uint8 public decimals = 18;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;

    event Transfer(address indexed from, address indexed to, uint256 value);

    constructor(uint256 _initialSupply) {
        totalSupply = _initialSupply;
        balanceOf[msg.sender] = _initialSupply;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        require(balanceOf[msg.sender] >= amount, "insufficient balance");
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }
}
```

---

## 4. Compile

```bash
npx hardhat compile
```

---

## 5. Deploy

Create `scripts/deploy.js`:

```javascript
const hre = require("hardhat");

async function main() {
  const Token = await hre.ethers.getContractFactory("Token");
  const token = await Token.deploy(hre.ethers.parseEther("1000000"));
  await token.waitForDeployment();
  console.log("Token deployed to:", await token.getAddress());
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
```

Deploy to Viri testnet:

```bash
npx hardhat run scripts/deploy.js --network viri
```

Example output:
```
Token deployed to: 0x8a9C...3fB7
```

Verify on the block explorer: `https://explorer.viri.me/address/0x8a9C...3fB7`

---

## 6. Interact

Create `scripts/interact.js`:

```javascript
const hre = require("hardhat");

async function main() {
  const [signer] = await hre.ethers.getSigners();
  const token = await hre.ethers.getContractAt("Token", "0x8a9C...3fB7");

  const balance = await token.balanceOf(signer.address);
  console.log("Balance:", hre.ethers.formatEther(balance), "VTT");

  const tx = await token.transfer("0xRecipientAddressHere", hre.ethers.parseEther("100"));
  await tx.wait();
  console.log("Transfer complete:", tx.hash);
}

main();
```

Run it:

```bash
npx hardhat run scripts/interact.js --network viri
```

---

## 7. Deploy with Raw JSON-RPC (No Hardhat)

For light-weight deployments:

```bash
# Get nonce
curl -s https://rpc.testnet.viri.me -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getTransactionCount","params":["0xYourAddress","latest"],"id":1}'

# Send raw transaction (hex-encoded RLP)
curl -s https://rpc.testnet.viri.me -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_sendRawTransaction","params":["0x..."],"id":1}'

# Get receipt
curl -s https://rpc.testnet.viri.me -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getTransactionReceipt","params":["0x..."],"id":1}'
```

Use [ethers.js](https://docs.ethers.org) or [web3.js](https://web3js.org) to construct and sign the raw transaction.

---

## 8. Account Abstraction (ERC-4337)

Viri supports ERC-4337 account abstraction. Deploy a smart wallet and submit user operations:

```typescript
import { ethers } from "ethers";

const provider = new ethers.JsonRpcProvider("https://rpc.testnet.viri.me");
const signer = new ethers.Wallet("0xYourPrivateKey", provider);

const userOp = {
  sender: "0x...",              // smart wallet address
  nonce: 0,
  initCode: "0x",               // "0x" if already deployed
  callData: "0x...",            // encoded function call
  callGasLimit: "0x10000",
  verificationGasLimit: "0x10000",
  preVerificationGas: "0x10000",
  maxFeePerGas: "0x...",
  maxPriorityFeePerGas: "0x...",
  paymaster: "0x",              // "0x" for self-pay
  signature: "0x...",
};

const hash = await provider.send("eth_sendUserOperation", [userOp, "0xEntryPointAddress"]);
console.log("UserOp hash:", hash);
```

> **Note**: Use `eth_getUserOperationReceipt` to check the status of a submitted user operation.

---

## Next Steps

- Explore the full [JSON-RPC API](/api/json-rpc)
- Read about [account abstraction](/architecture/layer2#account-abstraction)
- Join the [developer Discord](https://discord.gg/viri-chain)
