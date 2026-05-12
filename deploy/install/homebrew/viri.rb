class Viri < Formula
  desc "Viri blockchain node and CLI — 3-layer modular blockchain with native Account Abstraction, Multi-VM (WASM+EVM), and ZK privacy"
  homepage "https://viri-chain.org"
  license "MIT"
  version "0.1.0"

  if OS.mac?
    if Hardware::CPU.arm?
      url "https://github.com/viri-chain/viri/releases/download/v#{version}/viri-#{version}-darwin-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    else
      url "https://github.com/viri-chain/viri/releases/download/v#{version}/viri-#{version}-darwin-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  elsif OS.linux?
    if Hardware::CPU.arm?
      if Hardware::CPU.is_64_bit?
        url "https://github.com/viri-chain/viri/releases/download/v#{version}/viri-#{version}-linux-arm64.tar.gz"
        sha256 "0000000000000000000000000000000000000000000000000000000000000000"
      else
        url "https://github.com/viri-chain/viri/releases/download/v#{version}/viri-#{version}-linux-armv6.tar.gz"
        sha256 "0000000000000000000000000000000000000000000000000000000000000000"
      end
    else
      url "https://github.com/viri-chain/viri/releases/download/v#{version}/viri-#{version}-linux-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  def install
    bin.install "virid"
    bin.install "virictl"
  end

  test do
    system "#{bin}/virid", "--version"
    system "#{bin}/virictl", "version"
  end

  def caveats
    <<~EOS
      To start a validator node:
        virid --validator --chain-id 1337 --data-dir ~/.viri

      To create a wallet:
        virictl wallet create

      To view available options:
        virid --help
        virictl --help
    EOS
  end
end
