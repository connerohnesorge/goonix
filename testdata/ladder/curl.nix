let
  pkgs = import <nixpkgs> {
    localSystem = { system = "x86_64-linux"; };
    config = { };
    overlays = [ ];
  };
in pkgs.curl.drvPath
