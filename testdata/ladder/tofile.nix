(builtins.derivationStrict {
  name = "ladder-tofile";
  system = "x86_64-linux";
  builder = "/bin/sh";
  doc = builtins.toFile "ladder-tofile-doc" "hello store";
}).drvPath
