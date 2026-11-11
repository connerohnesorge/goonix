let
  a = ./path_plus_a;
  p = ./path_plus_y.txt;
in
builtins.toJSON {
  typeOfPathSuffix = builtins.typeOf (a + "/b");
  stringPlusPathType = builtins.typeOf ("pre-" + p);
  stringPlusPathCtxLen = builtins.length (builtins.attrNames (builtins.getContext ("pre-" + p)));
}
