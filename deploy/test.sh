#!/usr/bin/env bash
set -euo pipefail

export KUBECTL_APPLYSET=true

input=$(cat)

cmd_template=( helm template sgs "$SGS_WORKER_CHART_PATH" -f- )

cmd_apply=(
	kubectl apply
	--applyset workspacesets.sgs.snucse.org/sgs
	--prune -f-
)
if [[ "$SGS_WORKER_APPLY" != "true" ]]; then
	cmd_apply+=( --dry-run=client )
fi

cmd_register_harbor=( sgs-register-harbor )

<<<"$input" "${cmd_template[@]}" | "${cmd_apply[@]}"

if [[ "$SGS_WORKER_APPLY" = "true" ]]; then
	<<<"$input" "${cmd_register_harbor[@]}"
fi
