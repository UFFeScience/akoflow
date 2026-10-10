#!/bin/sh
# AKO-140: refresh tariffs without resetting the experimental CPU model.
set -eu
api=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
: "${AKOFLOW_API_TOKEN:?Load the API token without printing it}"
get() { curl -fsS -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$api/$1/"; }
post() { curl --fail-with-body -sS -H "Authorization: Bearer $AKOFLOW_API_TOKEN" -H 'Content-Type: application/json' --data-binary @- "$api/$1/"; }
catalog=$(get environments/gcp-environment/cloud-catalog | jq -c '{region,discoveredAt,disks,machines:[.machines[]|select(.providerTypeId=="e2-micro" or .providerTypeId=="e2-medium")]}')
targets=$(get environments/gcp-environment/cloud-capacity-targets)
for id in montage-gcp-e2-medium-1 montage-gcp-e2-medium-2 montage-gcp-fog-e2-micro-01 montage-gcp-fog-e2-micro-02 montage-gcp-fog-e2-micro-03 montage-gcp-fog-e2-micro-04 montage-gcp-fog-e2-micro-05 montage-gcp-fog-e2-micro-06 montage-gcp-fog-e2-micro-07 montage-gcp-fog-e2-micro-08 montage-gcp-fog-e2-micro-09 montage-gcp-fog-e2-micro-10; do
  original=$(get resources/$id)
  target=$(printf '%s' "$targets" | jq -ce --arg id "$id" '.[]|select(.id==$id)')
  printf '%s' "$target" | jq -e --argjson catalog "$catalog" '
    . as $target | $catalog.region==$target.region and
    ([$catalog.machines[] | select(.providerTypeId==$target.providerMachineType and .pricePerHour>0)]|length)==1 and
    ([$catalog.disks[] | select(.providerTypeId==$target.configuration.diskType and .pricePerGiBMonth>0)]|length)==1' >/dev/null
  updated=$(printf '%s' "$target" | jq --argjson catalog "$catalog" '.configuration.priceObservedAt=$catalog.discoveredAt | del(.createdAt)' | post environments/gcp-environment/cloud-capacity-targets)
  printf '%s' "$original" | jq --argjson target "$updated" '
    .pricePerSecond=($target.configuration.pricePerHour/3600) |
    .metadata.priceCurrency=$target.configuration.priceCurrency |
    .metadata.priceSource=$target.configuration.priceSource |
    .metadata.priceObservedAt=$target.configuration.priceObservedAt |
    .metadata.diskPricePerGiBMonth=$target.configuration.diskPricePerGiBMonth |
    .metadata.pricingStatus="estimated"' | post resources >/dev/null
  printf '%s' "$updated" | jq '{id,pricePerHour:.configuration.pricePerHour,diskPricePerGiBMonth:.configuration.diskPricePerGiBMonth,currency:.configuration.priceCurrency}'
done
