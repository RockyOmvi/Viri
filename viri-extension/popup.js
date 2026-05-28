const state = {
  account: {
    name: "Wallet 1",
    address: "0xa29539C21AC730F8c0501b719Ed7443bB4d16aB1"
  },
  chainId: "1986622057",
  rpcUrl: "https://rpc.viri.me",
  stakingContract: "",
  governanceContract: "",
  zkEndpoint: "",
  balance: "0.00"
};

function $(id) {
  return document.getElementById(id);
}

async function rpc(method, params = []) {
  const body = {
    jsonrpc: "2.0",
    id: Date.now(),
    method,
    params
  };
  const res = await fetch(state.rpcUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  });
  const data = await res.json();
  if (data.error) {
    throw new Error(data.error.message || "RPC error");
  }
  return data.result;
}

function toHex(value) {
  if (typeof value === "number") {
    return "0x" + value.toString(16);
  }
  return value;
}

function formatBalance(weiHex) {
  if (!weiHex) return "0.00";
  const wei = BigInt(weiHex);
  const whole = wei / 10n ** 18n;
  const frac = wei % 10n ** 18n;
  const fracStr = frac.toString().padStart(18, "0").slice(0, 4);
  return `${whole}.${fracStr}`;
}

async function refreshBalance() {
  try {
    const bal = await rpc("eth_getBalance", [state.account.address, "latest"]);
    state.balance = formatBalance(bal);
    $("balance").textContent = state.balance;
  } catch (err) {
    $("balance").textContent = "0.00";
  }
}

function showSettings(open) {
  const modal = $("settingsModal");
  modal.classList.toggle("active", open);
  modal.setAttribute("aria-hidden", open ? "false" : "true");
  if (open) {
    $("settingRpc").value = state.rpcUrl;
    $("settingChainId").value = state.chainId;
    $("settingAccount").value = state.account.address;
    $("settingStaking").value = state.stakingContract;
    $("settingGovernance").value = state.governanceContract;
    $("settingZk").value = state.zkEndpoint;
  }
}

function saveSettings() {
  state.rpcUrl = $("settingRpc").value.trim() || state.rpcUrl;
  state.chainId = $("settingChainId").value.trim() || state.chainId;
  state.account.address = $("settingAccount").value.trim() || state.account.address;
  state.stakingContract = $("settingStaking").value.trim();
  state.governanceContract = $("settingGovernance").value.trim();
  state.zkEndpoint = $("settingZk").value.trim();
  $("accountAddress").textContent = state.account.address;
  $("chainId").textContent = state.chainId;
  refreshBalance();
  showSettings(false);
  chrome.storage.local.set({ viriSettings: state });
}

async function loadSettings() {
  return new Promise(resolve => {
    chrome.storage.local.get(["viriSettings"], result => {
      if (result.viriSettings) {
        Object.assign(state, result.viriSettings);
      }
      resolve();
    });
  });
}

function initTabs() {
  document.querySelectorAll(".tab, .actions button").forEach(btn => {
    btn.addEventListener("click", () => {
      const tab = btn.getAttribute("data-tab");
      if (!tab) return;
      document.querySelectorAll(".tab").forEach(t => t.classList.toggle("active", t.dataset.tab === tab));
      document.querySelectorAll(".tab-panel").forEach(p => p.classList.toggle("active", p.id === `tab-${tab}`));
    });
  });
}

function setStatus(id, message) {
  const el = $(id);
  if (el) el.textContent = message;
}

async function sendTransaction() {
  setStatus("sendStatus", "Sending via faucet...");
  try {
    const to = $("sendTo").value.trim();
    const amount = $("sendAmount").value.trim();
    if (!to || !amount) {
      setStatus("sendStatus", "Provide destination and amount.");
      return;
    }
    const valueWei = BigInt(Math.floor(parseFloat(amount) * 1e18));
    const res = await fetch("https://faucet.viri.me/api/tx", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ to, value: "0x" + valueWei.toString(16) })
    });
    const data = await res.json();
    if (data.success) {
      setStatus("sendStatus", `Sent: ${data.tx_hash}`);
    } else {
      setStatus("sendStatus", `Error: ${data.error || "unknown"}`);
    }
  } catch (err) {
    setStatus("sendStatus", `Error: ${err.message}`);
  }
}

async function delegateStake() {
  setStatus("stakeStatus", "Preparing stake transaction...");
  try {
    const validator = $("stakeValidator").value.trim();
    const amount = $("stakeAmount").value.trim();
    const data = $("stakeData").value.trim();
    if (!validator || !amount) {
      setStatus("stakeStatus", "Provide validator and amount.");
      return;
    }
    setStatus("stakeStatus", "Staking: use MetaMask (no staking contract deployed).");
  } catch (err) {
    setStatus("stakeStatus", `Error: ${err.message}`);
  }
}

async function sendContractTx() {
  setStatus("contractStatus", "Submitting contract call...");
  try {
    const address = $("contractAddress").value.trim();
    const data = $("contractData").value.trim();
    if (!address || !data) {
      setStatus("contractStatus", "Provide address and call data.");
      return;
    }
    setStatus("contractStatus", "Contract: use MetaMask (no contracts deployed).");
  } catch (err) {
    setStatus("contractStatus", `Error: ${err.message}`);
  }
}

async function verifyZk() {
  setStatus("zkStatus", "Submitting proof...");
  try {
    if (!state.zkEndpoint) {
      setStatus("zkStatus", "Set ZK endpoint in Settings.");
      return;
    }
    const proof = $("zkProof").value.trim();
    const signals = $("zkSignals").value.trim();
    const res = await fetch(state.zkEndpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ proof, signals })
    });
    const data = await res.json();
    setStatus("zkStatus", data.valid ? "Proof verified" : "Invalid proof");
  } catch (err) {
    setStatus("zkStatus", `Error: ${err.message}`);
  }
}

async function castVote() {
  setStatus("govStatus", "Submitting vote...");
  try {
    const proposal = $("govProposal").value.trim();
    if (!proposal) {
      setStatus("govStatus", "Provide proposal id.");
      return;
    }
    setStatus("govStatus", "Governance: use MetaMask (no gov contract deployed).");
  } catch (err) {
    setStatus("govStatus", `Error: ${err.message}`);
  }
}

document.addEventListener("DOMContentLoaded", () => {
  loadSettings().then(() => {
    $("accountName").textContent = state.account.name;
    $("accountAddress").textContent = state.account.address;
    $("chainId").textContent = state.chainId;
    refreshBalance();
  });
  initTabs();

  $("sendBtn").addEventListener("click", sendTransaction);
  $("stakeBtn").addEventListener("click", delegateStake);
  $("contractBtn").addEventListener("click", sendContractTx);
  $("zkBtn").addEventListener("click", verifyZk);
  $("govBtn").addEventListener("click", castVote);
  $("settingsBtn").addEventListener("click", () => showSettings(true));
  $("closeSettings").addEventListener("click", () => showSettings(false));
  $("saveSettings").addEventListener("click", saveSettings);
});
