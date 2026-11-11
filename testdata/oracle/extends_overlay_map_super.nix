# Minimal repro: extends overlay with mapNamesToAttrs over super.${name} must not
# force every package while the fixpoint self is still blackholed (Nix 2.24.9 PASS).
let
  extends = overlay: f: final:
    let prev = f final; in prev // overlay final prev;

  gen = self: {
    pkg = self.cp ({ stdenv }: stdenv.mkDerivation {
      name = "pkg";
      builder = "builtin:derivation";
    });
    cp = fn: args: gen self // { };
  };

  setLicense = name: super: super.${name}.overrideAttrs (old: {
    meta = (old.meta or { }) // { license = "free"; };
  });

  mapNamesToAttrs = f: names:
    builtins.listToAttrs (map (n: { name = n; value = f n; }) names);

  overlay = self: super:
    { } // (mapNamesToAttrs (setLicense super) [ "pkg" ]);

  self = (extends overlay gen) self // { ok = 1; };
in self.ok
