package main

import (
	"fmt"
)

// completion scripts are static: the CLI surface is a fixed set of flags
// plus one optional directory argument, so generating them from a flag
// registry would add machinery without adding accuracy.

const bashCompletion = `# bash completion for gridfm
_gridfm() {
	local cur="${COMP_WORDS[COMP_WORDS - 1]}"
	local prev="${COMP_WORDS[COMP_WORDS - 1 - 1]}"

	case "$prev" in
	-icons) COMPREPLY=($(compgen -W "labels unicode nerdfont" -- "$cur")); return ;;
	-images) COMPREPLY=($(compgen -W "auto on off" -- "$cur")); return ;;
	-sort) COMPREPLY=($(compgen -W "name size modified type" -- "$cur")); return ;;
	-order) COMPREPLY=($(compgen -W "asc desc" -- "$cur")); return ;;
	-completions) COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur")); return ;;
	esac

	if [[ "$cur" == -* ]]; then
		COMPREPLY=($(compgen -W "-icons -images -sidebar -inspector -hidden \
			-sort -order -mouse -completions -help" -- "$cur"))
		return
	fi

	COMPREPLY=($(compgen -d -- "$cur")) # directories only
	complete -o filenames -F _gridfm gridfm
}
complete -o filenames -F _gridfm gridfm
`

const zshCompletion = `#compdef gridfm
# zsh completion for gridfm
_gridfm() {
	local -a flags
	flags=(
		'-icons[file type representation]:mode:(labels unicode nerdfont)'
		'-images[terminal image thumbnails]:mode:(auto on off)'
		'-sidebar[show the sidebar at start]'
		'-inspector[open the inspector panel at start]'
		'-hidden[show dot-prefixed entries]'
		'-sort[initial sort mode]:mode:(name size modified type)'
		'-order[initial sort direction]:dir:(asc desc)'
		'-mouse[enable mouse clicks and wheel scrolling]'
		'-completions[print a completion script]:shell:(bash zsh fish)'
		'-help[show usage]'
		'*:directory:_directories'
	)
	_arguments -S $flags
}
compdef _gridfm gridfm
`

const fishCompletion = `# fish completion for gridfm
complete -c gridfm -n '__fish_use_subcommand' -l icons -d 'file type representation' -r -a 'labels unicode nerdfont'
complete -c gridfm -n '__fish_use_subcommand' -l images -d 'terminal image thumbnails' -r -a 'auto on off'
complete -c gridfm -n '__fish_use_subcommand' -l sidebar -d 'show the sidebar at start'
complete -c gridfm -n '__fish_use_subcommand' -l inspector -d 'open the inspector panel at start'
complete -c gridfm -n '__fish_use_subcommand' -l hidden -d 'show dot-prefixed entries'
complete -c gridfm -n '__fish_use_subcommand' -l sort -d 'initial sort mode' -r -a 'name size modified type'
complete -c gridfm -n '__fish_use_subcommand' -l order -d 'initial sort direction' -r -a 'asc desc'
complete -c gridfm -n '__fish_use_subcommand' -l mouse -d 'enable mouse clicks and wheel scrolling'
complete -c gridfm -n '__fish_use_subcommand' -l completions -d 'print a completion script' -r -a 'bash zsh fish'
complete -c gridfm -n '__fish_use_subcommand' -l help -d 'show usage'
complete -c gridfm -k -n '__fish_use_subcommand' -a '(__fish_complete_directories)' -d 'directory'
`

// completionFor returns the completion script for a shell.
func completionFor(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletion, nil
	case "zsh":
		return zshCompletion, nil
	case "fish":
		return fishCompletion, nil
	default:
		return "", fmt.Errorf("unknown shell %q (want bash, zsh, or fish)", shell)
	}
}
