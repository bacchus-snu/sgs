{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    templ.url = "github:a-h/templ/v0.2.707";
    templ.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    {
      self,
      flake-utils,
      nixpkgs,
      templ,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ templ.overlays.default ];
        };
        selfPkgs = self.packages.${system};
      in
      {
        packages = {
          default = selfPkgs.sgs;
          sgs = pkgs.callPackage ./. { };
          sgs-docs = pkgs.callPackage ./docs { };
          image = pkgs.callPackage ./image.nix { inherit (selfPkgs) sgs; };
        };
        devShells.default = pkgs.callPackage ./shell.nix { inherit (selfPkgs) sgs sgs-docs; };
      }
    );
}
