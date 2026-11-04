(builtins.derivationStrict {
  name = "rung01-minimal";
  system = "x86_64-linux";
  builder = "/bin/sh";
}).drvPath
