class Chough < Formula
  desc "Fast, memory-efficient ASR CLI using sherpa-onnx"
  homepage "https://github.com/hyperpuncher/chough"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough-darwin-arm64"
      sha256 :no_check
    else
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough-darwin-amd64"
      sha256 :no_check
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough-linux-arm64"
      sha256 :no_check
    else
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough-linux-amd64"
      sha256 :no_check
    end
  end

  def install
    bin.install Dir["chough-*"].first => "chough"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/chough --version")
  end
end
