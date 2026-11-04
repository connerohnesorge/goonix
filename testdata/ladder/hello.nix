let
  pkgs = import <nixpkgs> {
    system = "x86_64-linux";
    config = { };
    overlays = [ ];
  };
in pkgs.hello
