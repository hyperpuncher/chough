class Chough < Formula
  desc "Fast ASR CLI using Parakeet TDT 0.6b V3"
  homepage "https://github.com/hyperpuncher/chough"
  version "0.1.4"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_darwin_arm64.tar.gz"
      sha256 :no_check
    else
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_darwin_x86_64.tar.gz"
      sha256 :no_check
    end
  end

  on_linux do
    url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_linux_x86_64.tar.gz"
    sha256 :no_check
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
