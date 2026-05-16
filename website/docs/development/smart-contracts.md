# Smart Contracts

Viri is fully EVM-compatible, so you can use standard Ethereum tooling.

## Tooling

- **Solidity**: Write contracts in Solidity
- **Hardhat/Foundry**: Use standard development frameworks
- **ethers.js/web3.js**: Use standard JavaScript libraries
- **Remix**: Use the web IDE for quick prototyping

## Deployment

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract MyToken {
    string public name = "My Token";
    string public symbol = "MTK";
    uint8 public decimals = 18;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;

    constructor(uint256 _initialSupply) {
        totalSupply = _initialSupply;
        balanceOf[msg.sender] = _initialSupply;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        require(balanceOf[msg.sender] >= amount, "insufficient balance");
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        return true;
    }
}
```

Deploy using the JSON-RPC API:

```bash
# Get nonce
curl https://rpc.testnet.viri.me -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getTransactionCount","params":["0x...","latest"],"id":1}'

# Send deployment transaction
curl https://rpc.testnet.viri.me -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_sendRawTransaction","params":["0x..."],"id":1}'

# Get receipt
curl https://rpc.testnet.viri.me -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getTransactionReceipt","params":["0x..."],"id":1}'
```

## Account Abstraction

Use ERC-4337 user operations for smart wallet functionality:

```typescript
const userOp = {
  sender: "0x...",
  nonce: 0,
  initCode: "0x",
  callData: "0x...",
  callGasLimit: "0x10000",
  verificationGasLimit: "0x10000",
  preVerificationGas: "0x10000",
  maxFeePerGas: "0x...",
  maxPriorityFeePerGas: "0x...",
  paymaster: "0x",
  signature: "0x..."
}
```
