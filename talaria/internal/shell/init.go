package shell

import "fmt"

// InitScript returns shell code that defines a thin wrapper around the
// talaria binary so `talaria new …` can change the interactive shell's
// working directory. Pattern: shell owns cwd; binary owns metadata.
func InitScript(shell string) (string, error) {
	switch shell {
	case "bash", "zsh", "sh":
		return bashZsh, nil
	case "fish":
		return fish, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (try: bash, zsh, fish)", shell)
	}
}

// bashZsh is sourced via: eval "$(talaria shell-init bash)"
//
// The wrapper:
//   - leaves non-new/enter subcommands alone
//   - for new/enter, captures the path from stdout and cds into it
//   - status messages stay on stderr (from the real binary)
const bashZsh = `# talaria shell integration
# eval "$(talaria shell-init bash)"   # or zsh

talaria() {
  case "$1" in
    new|enter|perdure)
      local path
      path="$(command talaria "$@")" || return $?
      if [ -n "$path" ] && [ -d "$path" ]; then
        cd -- "$path" || return $?
      fi
      ;;
    *)
      command talaria "$@"
      ;;
  esac
}
`

const fish = `# talaria shell integration
# talaria shell-init fish | source

function talaria
  switch $argv[1]
    case new enter perdure
      set -l path (command talaria $argv)
      or return $status
      if test -n "$path"; and test -d "$path"
        cd -- $path
        or return $status
      end
    case '*'
      command talaria $argv
  end
end
`
