{
  mkShell,
  namescale,

  gopls,
  nixfmt-rfc-style,
  markdownlint-cli,
}:

mkShell {
  inputsFrom = [ namescale ];

  buildInputs = [
    gopls
    nixfmt-rfc-style
    markdownlint-cli
  ];

  shellHook = ''
    export PS1="\033[0;31m[namescale]\033[0m $PS1"
  '';
}
