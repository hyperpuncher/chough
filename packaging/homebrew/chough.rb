class Chough < Formula
  desc "Fast ASR CLI using Parakeet TDT 0.6b V3"
  homepage "https://github.com/hyperpuncher/chough"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_Darwin_arm64.tar.gz"
      sha256 :no_check
    else
      odie "chough requires Apple Silicon (ARM64). Intel Macs are not supported."
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_Linux_arm64.tar.gz"
      sha256 :no_check
    else
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_Linux_x86_64.tar.gz"
      sha256 :no_check
    end
  end

  def install
    bin.install "chough"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/chough --version")
  end
end
