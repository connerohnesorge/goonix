let
  s = builtins.derivationStrict {
    name = "strict-attrnames-oracle";
    system = "x86_64-linux";
    builder = "/bin/sh";
    outputs = [ "out" "dev" ];
  };
in builtins.toJSON (builtins.attrNames s)
