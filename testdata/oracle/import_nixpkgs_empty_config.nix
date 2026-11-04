# PASS in real Nix: full import with empty config (same shape as TestImportNixpkgsOnly).
{ nixpkgsPath }:
import (nixpkgsPath + "/default.nix") {
  localSystem = { system = "x86_64-linux"; };
  config = { };
  overlays = [ ];
}
