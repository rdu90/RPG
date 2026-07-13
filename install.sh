#!/usr/bin/env bash
# Prepares a machine to build and work on RPG (github.com/rdu90/RPG).
#
# Idempotent by design: every step first checks whether it has anything to
# do, so running this a second (or fifth) time is quick and harmless. Safe
# to hand to someone who has never touched a terminal in anger.
set -euo pipefail

say() { printf '%s\n' "$*"; }

have() { command -v "$1" >/dev/null 2>&1; }

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# GOTOOLCHAIN=auto (the default since Go 1.21) lets 'go build' fetch the
# exact toolchain version go.mod asks for on its own. So the bootstrap Go
# from a package manager only needs to be new enough to have that feature —
# it does not need to already match go.mod's version.
required_min_go_minor=21

go_version_ok() {
	have go || return 1
	local v
	v="$(go env GOVERSION 2>/dev/null || true)" # e.g. "go1.24.0"
	[[ "$v" =~ ^go([0-9]+)\.([0-9]+) ]] || return 1
	local major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[2]}"
	((major > 1 || (major == 1 && minor >= required_min_go_minor)))
}

say "Good day. Let us see to the household's requirements for RPG."
say ""

pkg_manager=""
if have apt-get; then
	pkg_manager="apt"
elif have brew; then
	pkg_manager="brew"
fi

missing=()

if go_version_ok; then
	say "Go is already in residence, and properly current ($(go env GOVERSION))."
else
	if have go; then
		say "A Go is present, but a touch too dated ($(go version 2>/dev/null | awk '{print $3}')); we require at least 1.${required_min_go_minor}."
	else
		say "Go is not yet on the premises."
	fi
	case "$pkg_manager" in
	apt) missing+=(golang-go) ;;
	brew) missing+=(go) ;;
	esac
fi

if have git; then
	say "Git is already present and accounted for."
else
	say "Git is absent; we shall want that."
	case "$pkg_manager" in
	apt) missing+=(git) ;;
	brew) missing+=(git) ;;
	esac
fi

if have make; then
	say "Make stands ready."
else
	say "Make is missing, and the Makefile will find that disagreeable."
	case "$pkg_manager" in
	apt) missing+=(make) ;;
	brew) missing+=(make) ;;
	esac
fi

if have sqlite3; then
	say "The sqlite3 CLI is on hand, should you wish to inspect a save directly."
else
	say "The sqlite3 CLI is absent. Not strictly required — 'make db-query' manages without it — but 'make db-shell' will miss it."
	case "$pkg_manager" in
	apt) missing+=(sqlite3) ;;
	brew) missing+=(sqlite3) ;;
	esac
fi

say ""

if ((${#missing[@]} > 0)); then
	if [[ -z "$pkg_manager" ]]; then
		say "I regret that I do not recognise this household's package manager, and so cannot fetch: ${missing[*]}."
		say "Might I trouble you to install those yourself, then run this script again so I may confirm all is in order?"
		exit 1
	fi

	say "A moment, if you please — fetching: ${missing[*]}"
	case "$pkg_manager" in
	apt)
		sudo apt-get update -qq
		sudo apt-get install -y "${missing[@]}"
		;;
	brew)
		brew install "${missing[@]}"
		;;
	esac
	say ""
	say "That should do it. Allow me to check my work."
else
	say "Everything required was already in place. I have taken the liberty of double-checking, as is my habit."
fi

say ""

ok=1
have git || {
	say "git still eludes me."
	ok=0
}
have make || {
	say "make still eludes me."
	ok=0
}
go_version_ok || {
	say "A suitable Go still eludes me."
	ok=0
}

if ((ok == 0)); then
	say ""
	say "I cannot yet vouch for the household. Please see to the above and summon me again when ready."
	exit 1
fi

toolchain="$(go env GOTOOLCHAIN 2>/dev/null || echo auto)"
if [[ "$toolchain" == "local" ]]; then
	required_go="$(awk '/^go /{print $2; exit}' "$repo_root/go.mod")"
	say "One small note: your Go environment has GOTOOLCHAIN set to 'local'. RPG's go.mod asks for Go ${required_go}, and with GOTOOLCHAIN=local, 'go build' will decline to fetch it automatically. Running 'go env -u GOTOOLCHAIN' restores the default, should this arise."
	say ""
fi

say "Very good. Let us see whether the project itself agrees."
if (cd "$repo_root" && go build ./...); then
	say ""
	say "The project builds cleanly. All is in readiness."
	say "'make build', 'make run', 'make test', and 'make check' await your pleasure — 'make help' lists the rest."
else
	say ""
	say "The tools are all in place, but the build itself did not succeed — that, I'm afraid, is beyond my remit. Do have a look at the error above."
	exit 1
fi

say ""

# Optional: the Claude Code CLI. Unrelated to building RPG, but the user
# asked that this installer offer it, so it's a separate opt-in step rather
# than something bundled silently into the required toolchain above — it
# pipes a remote script into bash, which deserves an explicit yes.
if have claude; then
	say "One more thing: the Claude Code CLI is already in service."
elif [[ -t 0 ]]; then
	read -r -p "One more thing — shall I also install the Claude Code CLI? [Y/n] " reply
	reply="${reply:-Y}"
	if [[ "$reply" =~ ^[Yy] ]]; then
		say "Very good. Fetching it now."
		curl -fsSL https://claude.ai/install.sh | bash
	else
		say "As you wish. Should you change your mind, it's simply:"
		say "  curl -fsSL https://claude.ai/install.sh | bash"
	fi
else
	say "One more thing: the Claude Code CLI isn't installed. With no terminal to ask you properly, I'll refrain from fetching and running a remote script unattended. When convenient:"
	say "  curl -fsSL https://claude.ai/install.sh | bash"
fi
