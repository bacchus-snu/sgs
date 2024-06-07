#!/usr/bin/env bash
set -euo pipefail

export KUBECTL_APPLYSET=true

input=$(</dev/stdin)

cmd_template=( helm template sgs "$SGS_DEPLOY_CHART_PATH" -f- )
cmd_apply=( kubectl apply --applyset workspacesets.sgs.snucse.org/sgs --prune -f- )
cmd_register_harbor=( "$SGS_DEPLOY_REGHARBOR_PATH" )

if [[ "$SGS_DEPLOY_APPLY" != "true" ]]; then
	cmd_apply+=( --dry-run=client )
fi

<<<"$input" "${cmd_template[@]}" | "${cmd_apply[@]}"

if [[ "$SGS_DEPLOY_APPLY" = "true" ]]; then
	<<<"$input" "${cmd_register_harbor[@]}"
fi
