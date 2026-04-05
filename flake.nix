{
  description = "A development environment for cask";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }: let
    system = "x86_64-linux";
    pkgs = nixpkgs.legacyPackages.${system};
  in {
    devShells.${system}.default = pkgs.mkShell {
      buildInputs = with pkgs; [
        go
        gcc
        libcap
        golangci-lint
      ];
      HAKMES_PORT="9300";
      HAKMES_CASK_BASE="http://localhost:9201";
      HAKMES_CHUNK_SIZE="16777216";
      HAKMES_DB_PATH="hakmes.db";
    };
  };
}
