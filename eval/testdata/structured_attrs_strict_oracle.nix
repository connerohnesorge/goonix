builtins.derivationStrict {
  __structuredAttrs = true;
  name = "structured-attrs-oracle";
  system = "x86_64-linux";
  builder = "/bin/sh";
  nested = {
    foo = 1;
    bar = [ "a" null ];
    inner = { x = true; };
  };
}
