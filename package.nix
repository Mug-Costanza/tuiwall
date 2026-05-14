{ lib
, buildGoModule
, makeWrapper
, src
, version
, tmux
, git
, gh
, vhs
, python3
}:

buildGoModule {
  pname = "tuiwall";
  inherit version src;

  vendorHash = "sha256-JjXY4EargMoCMtmcUHyQwFRnMMyBUoZTm0ROgnwJ8wg=";

  subPackages = [ "cmd/tuiwall" ];

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/tuiwall \
      --prefix PATH : ${lib.makeBinPath [
        tmux    # core dependency - creates split-pane sessions
        git     # preset repository management
        gh      # GitHub CLI - community features (upload/search)
        vhs     # only for tuiwall record - demo GIF recording
        python3 # preset script runtime (standard library only)
      ]}
  '';

  meta = {
    description = "CLI wallpaper engine for the terminal via tmux split panes";
    homepage = "https://github.com/Mug-Costanza/tuiwall";
    license = lib.licenses.mit;
    mainProgram = "tuiwall";
    platforms = lib.platforms.unix;
  };
}
