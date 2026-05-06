{
  description = "Nix flake for tuiwall - tmux terminal wallpaper engine";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forEachSystem = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forEachSystem (pkgs:
        let tuiwall = pkgs.callPackage ./package.nix { };
        in {
          inherit tuiwall;
          default = tuiwall;
        });

      overlays.default = final: _prev: {
        tuiwall = final.callPackage ./package.nix { };
      };

      # Удобный nixosModule для тех, кто хочет добавить tuiwall одной строкой
      nixosModules.default = { pkgs, ... }: {
        nixpkgs.overlays = [ self.overlays.default ];
        environment.systemPackages = [ pkgs.tuiwall ];
      };
    };
}
