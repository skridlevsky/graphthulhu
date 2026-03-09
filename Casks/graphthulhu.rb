cask "graphthulhu" do
  version "0.4.0"

  # To update checksums for a new version, run:
  #   curl -sL https://github.com/skridlevsky/graphthulhu/releases/download/v<version>/checksums.txt
  on_macos do
    on_intel do
      url "https://github.com/skridlevsky/graphthulhu/releases/download/v#{version}/graphthulhu_#{version}_darwin_amd64.tar.gz"
      sha256 "bec4c6891fd6309a7114bf4a7ecb27e9fcc43b9b1532bbb1c637ebcb274d64eb"
    end
    on_arm do
      url "https://github.com/skridlevsky/graphthulhu/releases/download/v#{version}/graphthulhu_#{version}_darwin_arm64.tar.gz"
      sha256 "b4c962cb12f0bad00e047c1d5d6c5f00d827a094342d5689fc0b0ec3734670a4"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/skridlevsky/graphthulhu/releases/download/v#{version}/graphthulhu_#{version}_linux_amd64.tar.gz"
      sha256 "5de36682e46c02410c71aadea2c071f9b6bec6c4e183f4f6e8fa90c70ee47318"
    end
    on_arm do
      url "https://github.com/skridlevsky/graphthulhu/releases/download/v#{version}/graphthulhu_#{version}_linux_arm64.tar.gz"
      sha256 "794b026c28a4f5eeb2e82c8ef06a7078b01ad1dcedd373f7903e01d2c624c511"
    end
  end

  name "graphthulhu"
  desc "MCP knowledge graph server with Obsidian backend"
  homepage "https://github.com/skridlevsky/graphthulhu"

  livecheck do
    url :url
    strategy :github_latest
  end

  binary "graphthulhu"

  # No zap stanza required
end
