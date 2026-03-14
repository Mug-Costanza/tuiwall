class tuiwall < Formula
  desc "CLI wallpaper engine for the terminal"
  homepage "https://github.com/Mug-Costanza/tuiwall"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Mug-Costanza/tuiwall/releases/download/v0.1.0/tuiwall_Darwin_arm64.tar.gz"
      sha256 "sha256:5c6533b9ad888154e2a9aa55a006476bd424596f01ee633d3150e32ef730bed8"
    else
      url "https://github.com/Mug-Costanza/tuiwall/releases/download/v0.1.0/tuiwall_Darwin_x86_64.tar.gz"
      sha256 "sha256:3dfb652005a6459c2cfbd53b2dc04c6140353c6e662e2308f01ab270e5b667d6"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/Mug-Costanza/tuiwall/releases/download/v0.1.0/tuiwall_Linux_arm64.tar.gz"
      sha256 "sha256:59b68e5e045f769b936a5080aaddd9302aa2c3fb6d599b4b03f6237cca8e9c28"
    else
      url "https://github.com/Mug-Costanza/tuiwall/releases/download/v0.1.0/tuiwall_Linux_x86_64.tar.gz"
      sha256 "sha256:8f3114c4bd974480c69195447876c082c398a808d8c788618089136a4732b435"
    end
  end

  def install
    bin.install "tuiwall"
  end

  test do
    system "#{bin}/tuiwall", "--version"
  end
end
