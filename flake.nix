{
  description = "Nix flake for tuiwall - tmux terminal wallpaper engine";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "0.1.0";
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forEachSystem = f: nixpkgs.lib.genAttrs systems (system:
        f nixpkgs.legacyPackages.${system}
      );
    in
    {
      packages = forEachSystem (pkgs: rec {
        tuiwall = pkgs.callPackage ./package.nix { src = self; inherit version; };
        default = tuiwall;
      });

      overlays.default = final: _prev: {
        tuiwall = final.callPackage ./package.nix { src = self; inherit version; };
      };

      nixosModules.default = { lib, pkgs, config, ... }: {
        options.programs.tuiwall.enable = lib.mkEnableOption "tuiwall terminal wallpaper engine";

        config = lib.mkIf config.programs.tuiwall.enable {
          nixpkgs.overlays = [ self.overlays.default ];
          environment.systemPackages = [ pkgs.tuiwall ];
        };
      };
    };
}
