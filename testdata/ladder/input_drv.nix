let
  dep = builtins.derivationStrict {
    name = "ladder-input-drv-dep";
    system = "x86_64-linux";
    builder = "/bin/sh";
  };
in (builtins.derivationStrict {
  name = "ladder-input-drv-top";
  system = "x86_64-linux";
  builder = "/bin/sh";
  depOut = dep.out;
}).drvPath
