class Chough < Formula
  desc "Fast ASR CLI using Parakeet TDT 0.6b V3"
  homepage "https://github.com/hyperpuncher/chough"
  version "0.1.4"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_darwin_arm64.tar.gz"
      sha256 "8076b5504103853f66d5a8d0907edcfc624d7c306b9a924bba8fa65f1e97fda5"
    else
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_darwin_x86_64.tar.gz"
      sha256 "b89cfe033e8aee20f2bbe4c8d5139ed209ad6e0cf5729f1be1e1ebaf5c74b859"
    end
  end

  on_linux do
    url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_linux_x86_64.tar.gz"
    sha256 "7291e42e043bf8cd5a21551434c11537615eec0cd883bf685fcfa069eec6331c"
  end

  def install
    bin.install "chough-darwin-amd64" => "chough" if OS.mac? && Hardware::CPU.intel?
    bin.install "chough-darwin-arm64" => "chough" if OS.mac? && Hardware::CPU.arm?
    bin.install "chough-linux-amd64" => "chough" if OS.linux?

    # Install libs on macOS and Linux
    if OS.mac?
      lib.install Dir["*.dylib"]
    elsif OS.linux?
      lib.install Dir["*.so"]
    end
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/chough --version")
  end
end
