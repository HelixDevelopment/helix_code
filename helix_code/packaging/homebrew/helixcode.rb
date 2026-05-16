class Helixcode < Formula
  desc "Distributed AI Development Platform"
  homepage "https://helixcode.dev"
  url "https://github.com/helixcode/helixcode/releases/download/v3.0.0/helixcode-3.0.0.darwin.amd64.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "Proprietary"

  depends_on "go" => :build

  def install
    bin.install "helixcode"
    etc.install "config.yaml" => "helixcode/config.yaml.example"
    ohai "HelixCode installed!"
    ohai "Run 'helixcode server' to start the server"
    ohai "Run 'helixcode --help' for usage"
  end

  def caveats
    <<~EOS
      HelixCode requires configuration. Copy the example config:
        cp #{etc}/helixcode/config.yaml.example #{etc}/helixcode/config.yaml
      Then edit #{etc}/helixcode/config.yaml with your API keys.
    EOS
  end

  service do
    run [bin/"helixcode", "server"]
    keep_alive true
    log_path var/"log/helixcode.log"
    error_log_path var/"log/helixcode-error.log"
  end

  test do
    assert_match "helixcode version #{version}", shell_output("#{bin}/helixcode version")
  end
end
