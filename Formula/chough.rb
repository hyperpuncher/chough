class Chough < Formula
  desc "Fast ASR CLI using Parakeet TDT 0.6b V3"
  homepage "https://github.com/hyperpuncher/chough"
  version "0.1.4"
  license "MIT"

  depends_on "ffmpeg"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_darwin_arm64.tar.gz"
      sha256 "8076b5504103853f66d5a8d0907edcfc624d7c306b9a924bba8fa65f1e97fda5"
    else
      url "https://github.com/hyperpuncher/chough/releases/download/v#{version}/chough_v#{version}_darwin_x86_64.tar.gz"
      sha256 "b89cfe033e8aee20f2bbe4c8d5139ed209ad6e0cf5729f1be1e1ebaf5c74b859"
    end
  end

  def install
    bin.install Dir["chough*"].first => "chough"
    lib.install Dir["*.dylib"] if OS.mac?

    # Fix dylib references in the binary
    fix_dylib_references if OS.mac?
  end

  def fix_dylib_references
    dylibs = Dir["#{lib}/*.dylib"].map { |f| File.basename(f) }

    dylibs.each do |dylib|
      # Change from @loader_path to @rpath so it can find dylibs in lib/
      MachO::Tools.change_install_name(
        bin/"chough",
        "@loader_path/#{dylib}",
        "@rpath/#{dylib}",
      )
    rescue MachO::MachOError
      # Not found, skip
    end

    # Add rpath to the lib directory
    MachO::Tools.add_rpath(bin/"chough", opt_lib.to_s)
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/chough --version")
  end
end
